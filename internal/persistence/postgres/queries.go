package postgres

import (
	"cmp"
	"context"
	"fmt"
	"slices"
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
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	liberatorv1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

func Delete(ctx context.Context, input DeletePostgresInput) (*DeletePostgresPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	instance, err := client.Namespace(input.TeamSlug.String()).Get(ctx, input.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting PostgresInstance %q before deletion: %w", input.Name, err)
	}
	postgresName, _, err := unstructured.NestedString(instance.Object, "spec", "postgres")
	if err != nil || postgresName == "" {
		return nil, apierror.Errorf("PostgresInstance %q has no Postgres", input.Name)
	}
	postgresClient, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{
		Group: "nais.io", Version: "v1", Resource: "postgres",
	}))
	if err != nil {
		return nil, err
	}
	postgres, err := postgresClient.Namespace(input.TeamSlug.String()).Get(ctx, postgresName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting Postgres %q before instance deletion: %w", postgresName, err)
	}
	if err := ensureInstanceMayBeDeleted(instance, postgres); err != nil {
		return nil, err
	}
	if err := client.Namespace(input.TeamSlug.String()).Delete(ctx, input.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: ptr.To(instance.GetUID())}}); err != nil {
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

// ensureInstanceMayBeDeleted prevents an API request from marking the active
// instance as terminating. Pgrator independently blocks finalization as well.
func ensureInstanceMayBeDeleted(instance, postgres *unstructured.Unstructured) error {
	requested, _, err := unstructured.NestedString(postgres.Object, "spec", "activeInstance")
	if err != nil {
		return err
	}
	current, _, err := unstructured.NestedString(postgres.Object, "status", "activeInstance")
	if err != nil {
		return err
	}
	// Pgrator falls back to the Postgres name when neither field is set.
	if requested == "" && current == "" {
		current = postgres.GetName()
	}
	if instance.GetName() == requested || instance.GetName() == current {
		return apierror.Errorf("PostgresInstance %q is active and cannot be deleted", instance.GetName())
	}
	return nil
}

func GetForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName string) (*PostgresInstance, error) {
	if postgresName == "" {
		return nil, nil
	}
	postgres, err := GetPostgres(ctx, teamSlug, environmentName, postgresName)
	if err != nil {
		return nil, err
	}
	if postgres.ActiveInstance == nil {
		return nil, nil
	}
	return GetPostgresInstance(ctx, teamSlug, environmentName, *postgres.ActiveInstance)
}

// ListForWorkload resolves each Postgres use to the instance selected by that
// Postgres. A workload can use several databases, each with its own active instance.
func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, uses []liberatorv1.PostgresUse) ([]*PostgresInstance, error) {
	instances := make([]*PostgresInstance, 0, len(uses))
	for _, use := range uses {
		instance, err := GetForWorkload(ctx, teamSlug, environmentName, use.Name)
		if err != nil {
			return nil, err
		}
		if instance != nil {
			instances = append(instances, instance)
		}
	}
	slices.SortFunc(instances, func(a, b *PostgresInstance) int { return cmp.Compare(a.Name, b.Name) })
	return instances, nil
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

func GetPostgresInstanceByIdent(ctx context.Context, id ident.Ident) (*PostgresInstance, error) {
	teamSlug, environmentName, clusterName, err := parsePostgresInstanceIdent(id)
	if err != nil {
		return nil, err
	}

	return GetPostgresInstance(ctx, teamSlug, environmentName, clusterName)
}

func GetPostgresByIdent(ctx context.Context, id ident.Ident) (*Postgres, error) {
	teamSlug, environmentName, name, err := parseAccessIdent(id)
	if err != nil {
		return nil, err
	}
	return GetPostgres(ctx, teamSlug, environmentName, name)
}

func GetPostgresAccessByIdent(ctx context.Context, id ident.Ident) (*PostgresAccess, error) {
	teamSlug, environmentName, name, err := parseAccessIdent(id)
	if err != nil {
		return nil, err
	}

	return GetPostgresAccess(ctx, name, teamSlug, environmentName)
}

func postgresAccessGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgresaccesses"}
}

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

	return access, nil
}

func GetPostgresAccessConnection(ctx context.Context, input PostgresAccessConnectionInput) (*PostgresAccessConnectionDetails, error) {
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

	if err := loadPostgresAccessConnection(ctx, access, input, connection, credentialSecretName); err != nil {
		return nil, err
	}

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
	accessClient, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(postgresAccessGVR()))
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

func postgresAccessConnectionDetails(access *unstructured.Unstructured, now time.Time) (*PostgresAccessConnectionDetails, string, error) {
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

	if access.GetDeletionTimestamp() != nil {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	relayName, _, err := unstructured.NestedString(access.Object, "status", "relayAccess")
	if err != nil || relayName != access.GetName() {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	tokenSecret, _, err := unstructured.NestedString(access.Object, "status", "tokenSecret")
	if err != nil || tokenSecret == "" {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	role, _, err := unstructured.NestedString(access.Object, "status", "databaseRole")
	if err != nil || role == "" {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	return &PostgresAccessConnectionDetails{}, tokenSecret, nil
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

	relayName, _, _ := unstructured.NestedString(u.Object, "status", "relayAccess")

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
		RelayAccess:          strPtr(relayName),
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

func GetReadyPostgresInstance(ctx context.Context, teamSlug slug.Slug, environmentName, name string) (*PostgresInstance, error) {
	instance, err := GetPostgresInstance(ctx, teamSlug, environmentName, name)
	if err != nil {
		return nil, err
	}
	if instance.State != PostgresInstanceStateAvailable {
		return instance, nil
	}
	if _, err := GetPostgres(ctx, teamSlug, environmentName, instance.PostgresName); err != nil {
		return nil, fmt.Errorf("getting Postgres %q for instance %q: %w", instance.PostgresName, name, err)
	}
	// The pgrator reconciliation condition reflects the CNPG phase, but must be
	// corroborated with CNPG's own Ready condition before issuing access.
	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}))
	if err != nil {
		return nil, err
	}
	cluster, err := client.Namespace(teamSlug.String()).Get(ctx, nais_io_v1.CNPGClusterName(name), metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		instance.State = PostgresInstanceStateProgressing
		return instance, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting CNPG Cluster for PostgresInstance %q: %w", name, err)
	}
	conditions, found, err := unstructured.NestedSlice(cluster.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	if !found {
		instance.State = PostgresInstanceStateProgressing
		return instance, nil
	}
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if ok && condition["type"] == "Ready" && condition["status"] == "True" {
			return instance, nil
		}
	}
	instance.State = PostgresInstanceStateProgressing
	return instance, nil
}

func GetPostgresInstance(ctx context.Context, teamSlug slug.Slug, environmentName, name string) (*PostgresInstance, error) {
	return fromContext(ctx).postgresWatcher.Get(environmentName, teamSlug.String(), name)
}

func GetPostgres(ctx context.Context, teamSlug slug.Slug, environmentName, name string) (*Postgres, error) {
	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgres"}))
	if err != nil {
		return nil, err
	}
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return toPostgres(obj, environmentName)
}

const (
	postgresAccessAPIVersion = "nais.io/v1"
	defaultPostgresAccessTTL = time.Hour
	maxPostgresAccessTTL     = time.Hour
)

func CreatePostgresAccess(ctx context.Context, input CreatePostgresAccessInput) (*CreatePostgresAccessPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}
	accessTTL, err := input.accessTTL()
	if err != nil {
		return nil, err
	}

	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(postgresAccessGVR()))
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(accessTTL)
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
			Username:    authz.ActorFromContext(ctx).User.Identity(),
			AccessLevel: new(input.AccessLevel),
			ExpiresAt:   expiresAt,
			Reason:      input.Reason,
		},
	}); err != nil {
		return nil, err
	}

	return &CreatePostgresAccessPayload{Name: name, ExpiresAt: expiresAt}, nil
}

func (i CreatePostgresAccessInput) accessTTL() (time.Duration, error) {
	if i.TTL == "" {
		return defaultPostgresAccessTTL, nil
	}

	ttl, err := time.ParseDuration(i.TTL)
	if err != nil {
		return 0, fmt.Errorf("TTL must be a Go duration, for example %q", "4h")
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("TTL must be positive")
	}
	if ttl > maxPostgresAccessTTL {
		return 0, fmt.Errorf("TTL cannot exceed %s", maxPostgresAccessTTL)
	}
	return ttl, nil
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
		"postgresInstance": input.PostgresInstance,
		"username":         username,
		"accessLevel":      input.AccessLevel.CRDValue(),
		"expiresAt":        expiresAt.Format(time.RFC3339),
	}
	return res
}

func WorkloadsForInstance(ctx context.Context, teamSlug slug.Slug, environmentName, instanceName string) []workload.Workload {
	instance, err := GetPostgresInstance(ctx, teamSlug, environmentName, instanceName)
	if err != nil {
		return nil
	}
	postgres, err := GetPostgres(ctx, teamSlug, environmentName, instance.PostgresName)
	if err != nil || postgres.ActiveInstance == nil || *postgres.ActiveInstance != instanceName {
		return nil
	}
	apps := application.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)
	jobs := job.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)
	ret := make([]workload.Workload, 0)
	for _, app := range apps {
		if app.Spec != nil && app.Spec.Uses != nil && slices.ContainsFunc(app.Spec.Uses.Postgres, func(use liberatorv1.PostgresUse) bool { return use.Name == instance.PostgresName }) {
			ret = append(ret, app)
		}
	}
	for _, j := range jobs {
		if j.Spec != nil && j.Spec.Uses != nil && slices.ContainsFunc(j.Spec.Uses.Postgres, func(use liberatorv1.PostgresUse) bool { return use.Name == instance.PostgresName }) {
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
