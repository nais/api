package postgres

import (
	"cmp"
	"context"
	"fmt"
	"hash/crc32"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func Delete(ctx context.Context, input DeletePostgresInput) (*DeletePostgresPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := fromContext(ctx).postgresWatcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, input.TeamSlug.String())
	if err != nil {
		return nil, err
	}

	obj, err := client.Get(ctx, input.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	allowDeletion, _, err := unstructured.NestedBool(obj.Object, "spec", "cluster", "allowDeletion")
	if err != nil {
		return nil, err
	}
	if !allowDeletion {
		if err := unstructured.SetNestedField(obj.Object, true, "spec", "cluster", "allowDeletion"); err != nil {
			return nil, err
		}
		if _, err = client.Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("enabling deletion: %w", err)
		}
	}

	if err := fromContext(ctx).postgresWatcher.Delete(ctx, input.EnvironmentName, input.TeamSlug.String(), input.Name); err != nil {
		return nil, err
	}

	if err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionDeleted,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    input.Name,
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
	}); err != nil {
		return nil, err
	}

	return &DeletePostgresPayload{PostgresDeleted: new(true)}, nil
}

func GetForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, clusterName string) (*PostgresInstance, error) {
	if clusterName == "" {
		return nil, nil
	}

	return GetPostgres(ctx, teamSlug, environmentName, clusterName)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *PostgresInstanceOrder, filter *PostgresInstanceFilter) (*PostgresInstanceConnection, error) {
	all := ListAllForTeam(ctx, teamSlug, filter)

	if orderBy == nil {
		orderBy = &PostgresInstanceOrder{
			Field:     PostgresInstanceOrderFieldName,
			Direction: model.OrderDirectionAsc,
		}
	}

	return SortFilterPostgresInstance.PaginatedList(ctx, all, page, orderBy.Field, orderBy.Direction, filter), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug, filter *PostgresInstanceFilter) []*PostgresInstance {
	all := fromContext(ctx).postgresWatcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func CountForTeam(ctx context.Context, teamSlug slug.Slug) int {
	return len(fromContext(ctx).postgresWatcher.GetByNamespace(teamSlug.String()))
}

func GetPostgresByIdent(ctx context.Context, id ident.Ident) (*PostgresInstance, error) {
	teamSlug, environmentName, clusterName, err := parsePostgresInstanceIdent(id)
	if err != nil {
		return nil, err
	}

	return GetPostgres(ctx, teamSlug, environmentName, clusterName)
}

func GetPostgresAccessByIdent(ctx context.Context, id ident.Ident) (*PostgresAccess, error) {
	teamSlug, environmentName, name, err := parseAccessIdent(id)
	if err != nil {
		return nil, err
	}

	return GetPostgresAccess(ctx, name, teamSlug, environmentName)
}

const (
	postgresAccessResource = "postgresaccesses"
	postgresAccessGroup    = "nais.io"
)

// GetPostgresAccess returns a personal PostgresAccess status. Connection
// credentials are deliberately available only through GetPostgresAccessConnection.
func GetPostgresAccess(ctx context.Context, name string, teamSlug slug.Slug, environmentName string) (*PostgresAccess, error) {
	if err := authz.CanGrantPostgresAccess(ctx, teamSlug); err != nil {
		return nil, err
	}

	u, err := getPostgresAccessResource(ctx, name, teamSlug, environmentName)
	if err != nil {
		return nil, err
	}

	access, err := toPostgresAccess(u, teamSlug, environmentName)
	if err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx)
	if actor == nil || access.Username != actor.User.Identity() {
		return nil, apierror.Errorf("PostgresAccess %q not found", name)
	}

	return access, nil
}

func GetPostgresAccessConnection(ctx context.Context, input PostgresAccessConnectionInput) (*PostgresAccessConnectionPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}
	if err := authz.CanGrantPostgresAccess(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	access, err := getPostgresAccessResource(ctx, input.Name, input.TeamSlug, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	username, _, err := unstructured.NestedString(access.Object, "spec", "username")
	if err != nil {
		return nil, fmt.Errorf("reading PostgresAccess %q username: %w", input.Name, err)
	}
	actor := authz.ActorFromContext(ctx)
	if actor == nil || username == "" || actor.User.Identity() != username {
		return nil, authz.ErrUnauthorized
	}

	connection, credentialSecretName, err := postgresAccessConnectionDetails(access, time.Now())
	if err != nil {
		return nil, err
	}

	secretClient, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{
		Version:  "v1",
		Resource: "secrets",
	}))
	if err != nil {
		return nil, fmt.Errorf("creating credential Secret client: %w", err)
	}
	secret, err := secretClient.Namespace(input.TeamSlug.String()).Get(ctx, credentialSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, apierror.Errorf("credentials for PostgresAccess %q are not available", input.Name)
		}
		return nil, fmt.Errorf("getting credential Secret for PostgresAccess %q: %w", input.Name, err)
	}

	password, caCertificate, err := postgresAccessConnectionSecret(secret)
	if err != nil {
		return nil, err
	}
	connection.Password = password
	connection.CACertificate = caCertificate

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionGetPersonalAccessConnection,
		Actor:           actor.User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    input.Name,
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
		Data:            PostgresPersonalAccessConnectionActivityLogEntryData{},
	}); err != nil {
		return nil, err
	}

	return connection, nil
}

func getPostgresAccessResource(ctx context.Context, name string, teamSlug slug.Slug, environmentName string) (*unstructured.Unstructured, error) {
	accessClient, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{
		Group:    postgresAccessGroup,
		Version:  "v1",
		Resource: postgresAccessResource,
	}))
	if err != nil {
		return nil, fmt.Errorf("creating postgresaccess client: %w", err)
	}
	u, err := accessClient.Namespace(teamSlug.String()).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, apierror.Errorf("PostgresAccess %q not found", name)
		}
		return nil, fmt.Errorf("getting PostgresAccess %q: %w", name, err)
	}
	return u, nil
}

func postgresAccessConnectionDetails(access *unstructured.Unstructured, now time.Time) (*PostgresAccessConnectionPayload, string, error) {
	expiresAt, _, err := unstructured.NestedString(access.Object, "spec", "expiresAt")
	if err != nil {
		return nil, "", fmt.Errorf("reading PostgresAccess %q expiry: %w", access.GetName(), err)
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, "", apierror.Errorf("PostgresAccess %q has an invalid expiry", access.GetName())
	}
	if !expires.After(now) {
		return nil, "", apierror.Errorf("PostgresAccess %q has expired", access.GetName())
	}
	if !postgresAccessIsReady(access.Object) {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}

	credentialSecretName, _, err := unstructured.NestedString(access.Object, "status", "credentialSecretName")
	if err != nil || credentialSecretName == "" {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	serverName, _, err := unstructured.NestedString(access.Object, "status", "serverName")
	if err != nil || serverName == "" {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	endpoint, _, err := unstructured.NestedString(access.Object, "status", "tunnel", "endpoint")
	if err != nil || endpoint == "" {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	gatewayPublicKey, _, err := unstructured.NestedString(access.Object, "status", "tunnel", "gatewayPublicKey")
	if err != nil || gatewayPublicKey == "" {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}

	return &PostgresAccessConnectionPayload{
		ServerName: serverName,
		Tunnel: PostgresAccessConnectionTunnel{
			Endpoint: endpoint, GatewayPublicKey: gatewayPublicKey,
		},
	}, credentialSecretName, nil
}

func postgresAccessIsReady(obj map[string]any) bool {
	conditions, found, err := unstructured.NestedSlice(obj, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, c := range conditions {
		condition, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] == "Ready" && condition["status"] == string(metav1.ConditionTrue) {
			return true
		}
	}
	return false
}

func postgresAccessConnectionSecret(secret *unstructured.Unstructured) (password, caCertificate string, err error) {
	var typed corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(secret.Object, &typed); err != nil {
		return "", "", fmt.Errorf("converting credential Secret %q: %w", secret.GetName(), err)
	}
	password = string(typed.Data[corev1.BasicAuthPasswordKey])
	caCertificate = string(typed.Data["ca.crt"])
	if password == "" || caCertificate == "" {
		return "", "", apierror.Errorf("credentials for PostgresAccess are incomplete")
	}
	return password, caCertificate, nil
}

func toPostgresAccess(u *unstructured.Unstructured, teamSlug slug.Slug, environmentName string) (*PostgresAccess, error) {
	name := u.GetName()
	postgresInstance, _, _ := unstructured.NestedString(u.Object, "spec", "postgresInstance")
	username, _, _ := unstructured.NestedString(u.Object, "spec", "username")
	levelStr, _, _ := unstructured.NestedString(u.Object, "spec", "accessLevel")
	expiresStr, _, _ := unstructured.NestedString(u.Object, "spec", "expiresAt")

	expiresAt, err := time.Parse(time.RFC3339, expiresStr)
	if err != nil {
		return nil, fmt.Errorf("parsing expiresAt for PostgresAccess %q: %w", name, err)
	}

	level := PostgresAccessLevel(strings.ToUpper(levelStr))
	if !level.IsValid() {
		return nil, fmt.Errorf("invalid accessLevel %q for PostgresAccess %q", levelStr, name)
	}

	state, message := postgresAccessState(u.Object, expiresAt)

	var tunnel *PostgresAccessTunnel
	if t, ok, _ := unstructured.NestedStringMap(u.Object, "status", "tunnel"); ok && t["name"] != "" {
		tunnel = &PostgresAccessTunnel{
			Name:             t["name"],
			Endpoint:         strPtr(t["endpoint"]),
			GatewayPublicKey: strPtr(t["gatewayPublicKey"]),
		}
	}

	return &PostgresAccess{
		Name:                 name,
		TeamSlug:             teamSlug,
		EnvironmentName:      environmentName,
		PostgresInstanceName: postgresInstance,
		Username:             username,
		AccessLevel:          level,
		ExpiresAt:            expiresAt,
		State:                state,
		Message:              strPtr(message),
		Tunnel:               tunnel,
	}, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func postgresAccessState(obj map[string]any, expiresAt time.Time) (PostgresAccessState, string) {
	if !expiresAt.IsZero() && expiresAt.Before(time.Now()) {
		return PostgresAccessStateExpired, "access has expired"
	}

	conditions, found, err := unstructured.NestedSlice(obj, "status", "conditions")
	if err != nil || !found {
		return PostgresAccessStatePending, "waiting for controller"
	}

	for _, c := range conditions {
		condition, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, _ := condition["type"].(string)
		if condType != "Ready" {
			continue
		}

		status, _ := condition["status"].(string)
		reason, _ := condition["reason"].(string)
		message, _ := condition["message"].(string)

		if status == string(metav1.ConditionTrue) {
			return PostgresAccessStateReady, message
		}
		if reason == "UnsupportedAccessLevel" {
			return PostgresAccessStateFailed, message
		}
		return PostgresAccessStatePending, message
	}

	return PostgresAccessStatePending, "waiting for controller"
}

func GetPostgres(ctx context.Context, teamSlug slug.Slug, environmentName string, clusterName string) (*PostgresInstance, error) {
	return fromContext(ctx).postgresWatcher.Get(environmentName, teamSlug.String(), clusterName)
}

func GetAuditURL(ctx context.Context, audit *PostgresInstanceAudit) (*string, error) {
	if audit == nil || !audit.Enabled {
		return nil, nil
	}

	auditProjectID, location := GetAuditLogConfig(ctx)
	if auditProjectID == "" || location == "" {
		return nil, nil
	}

	teamEnv, err := team.GetTeamEnvironment(ctx, audit.TeamSlug, audit.EnvironmentName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team environment for audit URL (team=%s, env=%s): %w", audit.TeamSlug, audit.EnvironmentName, err)
	}
	if teamEnv.GCPProjectID == nil || *teamEnv.GCPProjectID == "" {
		return nil, nil
	}

	databaseProjectID := *teamEnv.GCPProjectID
	databaseID := fmt.Sprintf("%s:%s", databaseProjectID, audit.InstanceName)
	query := fmt.Sprintf("labels.databaseId=\"%s\"", databaseID)
	storageScope := fmt.Sprintf("storage,projects/%s/locations/%s/buckets/%s-%s/views/_AllLogs", auditProjectID, location, audit.TeamSlug.String(), audit.EnvironmentName)
	logURL := fmt.Sprintf("https://console.cloud.google.com/logs/query;query=%s;storageScope=%s?project=%s",
		url.QueryEscape(query),
		url.QueryEscape(storageScope),
		databaseProjectID,
	)
	return &logURL, nil
}

const postgresAccessAPIVersion = "nais.io/v1"

func CreatePostgresAccess(ctx context.Context, input CreatePostgresAccessInput) (*CreatePostgresAccessPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    "nais.io",
		Version:  "v1",
		Resource: "postgresaccesses",
	}
	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(gvr))
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(time.Hour)
	name := fmt.Sprintf("postgres-access-%s", uuid.NewString()[:8])
	res := newPostgresAccessResource(input, authz.ActorFromContext(ctx).User.Identity(), name, expiresAt)

	if _, err := client.Namespace(input.TeamSlug.String()).Create(ctx, res, metav1.CreateOptions{}); err != nil {
		return nil, err
	}

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionCreatePersonalAccess,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    input.PostgresInstance,
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
		Data: PostgresPersonalAccessCreatedActivityLogEntryData{
			Username:  authz.ActorFromContext(ctx).User.Identity(),
			ExpiresAt: expiresAt,
			Reason:    input.Reason,
		},
	}); err != nil {
		return nil, err
	}

	return &CreatePostgresAccessPayload{Name: name, ExpiresAt: expiresAt}, nil
}

func newPostgresAccessResource(input CreatePostgresAccessInput, username, name string, expiresAt time.Time) *unstructured.Unstructured {
	res := &unstructured.Unstructured{}
	res.SetAPIVersion(postgresAccessAPIVersion)
	res.SetKind("PostgresAccess")
	res.SetName(name)
	res.SetNamespace(input.TeamSlug.String())
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, username))
	kubernetes.SetManagedByConsoleLabel(res)
	res.Object["spec"] = map[string]any{
		"postgresInstance":         input.PostgresInstance,
		"username":                 username,
		"accessLevel":              input.AccessLevel.CRDValue(),
		"expiresAt":                expiresAt.Format(time.RFC3339),
		"clientWireGuardPublicKey": input.ClientWireGuardPublicKey,
	}
	return res
}

func GrantPostgresAccess(ctx context.Context, input GrantPostgresAccessInput) error {
	err := input.Validate(ctx)
	if err != nil {
		return err
	}

	namespace := fmt.Sprintf("pg-%s", input.TeamSlug.String())
	name, err := resourceNamer(input.TeamSlug, input.Grantee, input.ClusterName)
	if err != nil {
		return err
	}

	annotations := make(map[string]string)
	d, err := time.ParseDuration(input.Duration)
	if err != nil {
		return fmt.Errorf("parsing TTL: %w", err)
	}
	until := time.Now().Add(d)

	labels := make(map[string]string)
	labels["euthanaisa.nais.io/kill-after"] = strconv.FormatInt(until.Unix(), 10)
	labels["postgres.data.nais.io/name"] = input.ClusterName

	err = createRole(ctx, input, name, namespace, annotations, labels)
	if err != nil {
		return err
	}

	err = createRoleBinding(ctx, input, name, namespace, annotations, labels)
	if err != nil {
		return err
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionGrantAccess,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    input.ClusterName,
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
		Data: PostgresGrantAccessActivityLogEntryData{
			Grantee: input.Grantee,
			Until:   until,
		},
	})
}

func createRoleBinding(ctx context.Context, input GrantPostgresAccessInput, name string, namespace string, annotations map[string]string, labels map[string]string) error {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "rolebindings",
	}
	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(gvr))
	if err != nil {
		return err
	}
	namespacedClient := client.Namespace(namespace)

	res := &unstructured.Unstructured{}
	res.SetAPIVersion(gvr.GroupVersion().String())
	res.SetKind("RoleBinding")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(annotations, authz.ActorFromContext(ctx).User.Identity()))
	res.SetLabels(labels)
	kubernetes.SetManagedByConsoleLabel(res)

	res.Object["roleRef"] = map[string]any{
		"apiGroup": "rbac.authorization.k8s.io",
		"kind":     "Role",
		"name":     name,
	}

	res.Object["subjects"] = []any{
		map[string]any{
			"kind": "User",
			"name": input.Grantee,
		},
	}

	return createOrUpdateResource(ctx, res, namespacedClient)
}

func createRole(ctx context.Context, input GrantPostgresAccessInput, name string, namespace string, annotations map[string]string, labels map[string]string) error {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "roles",
	}

	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(gvr))
	if err != nil {
		return err
	}
	namespacedClient := client.Namespace(namespace)

	res := &unstructured.Unstructured{}
	res.SetAPIVersion(gvr.GroupVersion().String())
	res.SetKind("Role")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(annotations, authz.ActorFromContext(ctx).User.Identity()))
	res.SetLabels(labels)
	kubernetes.SetManagedByConsoleLabel(res)

	res.Object["rules"] = []any{
		map[string]any{
			"apiGroups": []any{""},
			"resources": []any{"pods"},
			"verbs":     []any{"get", "list", "watch"},
			"resourceNames": []any{
				fmt.Sprintf("%s-0", input.ClusterName),
				fmt.Sprintf("%s-1", input.ClusterName),
				fmt.Sprintf("%s-2", input.ClusterName),
			},
		},
		map[string]any{
			"apiGroups": []any{""},
			"resources": []any{"pods/portforward"},
			"verbs":     []any{"get", "list", "watch", "create"},
			"resourceNames": []any{
				fmt.Sprintf("%s-0", input.ClusterName),
				fmt.Sprintf("%s-1", input.ClusterName),
				fmt.Sprintf("%s-2", input.ClusterName),
			},
		},
	}

	return createOrUpdateResource(ctx, res, namespacedClient)
}

func createOrUpdateResource(ctx context.Context, res *unstructured.Unstructured, client dynamic.ResourceInterface) error {
	_, err := client.Create(ctx, res, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			_, err = client.Update(ctx, res, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func resourceNamer(teamSlug slug.Slug, grantee string, name string) (string, error) {
	hasher := crc32.NewIEEE()
	_, err := fmt.Fprintf(hasher, "%s-%s-%s", teamSlug.String(), grantee, name)
	if err != nil {
		return "", err
	}
	hashStr := fmt.Sprintf("%08x", hasher.Sum32())
	return fmt.Sprintf("pg-grant-%s", hashStr), nil
}

func WorkloadsForInstance(ctx context.Context, teamSlug slug.Slug, environmentName, clusterName string) []workload.Workload {
	apps := application.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)
	jobs := job.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)

	ret := make([]workload.Workload, 0)
	for _, app := range apps {
		if app.Spec != nil && app.Spec.Postgres != nil && app.Spec.Postgres.ClusterName == clusterName {
			ret = append(ret, app)
		}
	}

	for _, j := range jobs {
		if j.Spec != nil && j.Spec.Postgres != nil && j.Spec.Postgres.ClusterName == clusterName {
			ret = append(ret, j)
		}
	}

	slices.SortFunc(ret, func(a, b workload.Workload) int {
		if a.GetName() != b.GetName() {
			return cmp.Compare(a.GetName(), b.GetName())
		}

		return cmp.Compare(a.GetType(), b.GetType())
	})

	return ret
}
