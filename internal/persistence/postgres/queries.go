package postgres

import (
	"cmp"
	"context"
	"errors"
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
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
)

func Delete(ctx context.Context, input DeletePostgresBranchInput) (*DeletePostgresBranchPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	objectName := nais_io_v1.PostgresBranchObjectName(input.Postgres, input.Branch)
	instance, err := client.Namespace(input.TeamSlug.String()).Get(ctx, objectName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("PostgresBranch %q not found in Postgres %q", input.Branch, input.Postgres)
	}
	if err != nil {
		return nil, fmt.Errorf("getting PostgresBranch %q before deletion: %w", input.Branch, err)
	}
	branch, err := toPostgresBranch(instance, input.EnvironmentName)
	if err != nil || branch.PostgresName != input.Postgres || branch.Name != input.Branch {
		return nil, apierror.Errorf("PostgresBranch %q not found in Postgres %q", input.Branch, input.Postgres)
	}
	postgresClient, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{
		Group: "nais.io", Version: "v1", Resource: "postgres",
	}))
	if err != nil {
		return nil, err
	}
	postgres, err := postgresClient.Namespace(input.TeamSlug.String()).Get(ctx, input.Postgres, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting Postgres %q before instance deletion: %w", input.Postgres, err)
	}
	if err := ensureInstanceMayBeDeleted(input.Branch, postgres); err != nil {
		return nil, err
	}
	if err := client.Namespace(input.TeamSlug.String()).Delete(ctx, objectName, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: ptr.To(instance.GetUID())}}); err != nil {
		return nil, err
	}

	if err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionDeleted,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    input.Postgres,
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
	}); err != nil {
		return nil, err
	}

	return &DeletePostgresBranchPayload{PostgresBranchDeleted: new(true)}, nil
}

// ensureInstanceMayBeDeleted prevents an API request from marking the active
// instance as terminating. Pgrator independently blocks finalization as well.
func ensureInstanceMayBeDeleted(branch string, postgres *unstructured.Unstructured) error {
	obj, err := kubernetes.ToConcrete[nais_io_v1.Postgres](postgres)
	if err != nil {
		return err
	}
	requested := obj.Spec.ActiveBranch
	current := ""
	if obj.Status != nil {
		current = obj.Status.ActiveBranch
	}
	// Pgrator selects main when neither field is set.
	if requested == "" && current == "" {
		current = nais_io_v1.DefaultBranchName
	}
	if branch == requested || branch == current {
		return apierror.Errorf("PostgresBranch %q is active and cannot be deleted", branch)
	}
	return nil
}

func GetForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName string) (*PostgresBranch, error) {
	if postgresName == "" {
		return nil, nil
	}
	postgres, err := GetPostgres(ctx, teamSlug, environmentName, postgresName)
	if err != nil {
		return nil, err
	}
	if postgres.ActiveBranch == nil {
		return nil, nil
	}
	return GetPostgresBranch(ctx, teamSlug, environmentName, postgresName, *postgres.ActiveBranch)
}

// ListForWorkload resolves each Postgres use to the instance selected by that
// Postgres. A workload can use several databases, each with its own active instance.
func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, uses []liberatorv1.PostgresUse) ([]*PostgresBranch, error) {
	instances := make([]*PostgresBranch, 0, len(uses))
	for _, use := range uses {
		instance, err := GetForWorkload(ctx, teamSlug, environmentName, use.Name)
		if err != nil {
			return nil, err
		}
		if instance != nil {
			instances = append(instances, instance)
		}
	}
	slices.SortFunc(instances, func(a, b *PostgresBranch) int {
		if a.Name != b.Name {
			return cmp.Compare(a.Name, b.Name)
		}
		return cmp.Compare(a.PostgresName, b.PostgresName)
	})
	return instances, nil
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *PostgresBranchOrder, filter *PostgresBranchFilter) (*PostgresBranchConnection, error) {
	all := ListAllForTeam(ctx, teamSlug, filter)

	if orderBy == nil {
		orderBy = &PostgresBranchOrder{
			Field:     PostgresBranchOrderFieldName,
			Direction: model.OrderDirectionAsc,
		}
	}

	return SortFilterPostgresBranch.PaginatedList(ctx, all, page, orderBy.Field, orderBy.Direction, filter), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug, filter *PostgresBranchFilter) []*PostgresBranch {
	all := fromContext(ctx).postgresBranchWatcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func CountForTeam(ctx context.Context, teamSlug slug.Slug) int {
	return len(fromContext(ctx).postgresBranchWatcher.GetByNamespace(teamSlug.String()))
}

func GetPostgresBranchByIdent(ctx context.Context, id ident.Ident) (*PostgresBranch, error) {
	teamSlug, environmentName, postgresName, branchName, err := parsePostgresBranchIdent(id)
	if err != nil {
		return nil, err
	}

	return GetPostgresBranch(ctx, teamSlug, environmentName, postgresName, branchName)
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

	resource, err := kubernetes.ToConcrete[nais_io_v1.PostgresAccess](access)
	if err != nil {
		return nil, fmt.Errorf("converting PostgresAccess %q: %w", input.Name, err)
	}
	actor := authz.ActorFromContext(ctx)
	if actor == nil || resource.Spec.Username == "" || actor.User.Identity() != resource.Spec.Username {
		return nil, authz.ErrUnauthorized
	}

	// Status lives in PostgresAccess.state. Until the access is ready there are no
	// connection materials, which is not an error for a caller polling for readiness.
	if state, _ := postgresAccessState(resource, resource.Spec.ExpiresAt.Time); state != PostgresAccessStateReady || access.GetDeletionTimestamp() != nil {
		return nil, nil
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
	accessClient, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(postgresAccessGVR()))
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
	obj, err := kubernetes.ToConcrete[nais_io_v1.PostgresAccess](access)
	if err != nil {
		return nil, "", fmt.Errorf("converting PostgresAccess %q: %w", access.GetName(), err)
	}
	if obj.Spec.ExpiresAt.IsZero() {
		return nil, "", apierror.Errorf("PostgresAccess %q has an invalid expiry", access.GetName())
	}
	if !obj.Spec.ExpiresAt.After(now) {
		return nil, "", apierror.Errorf("PostgresAccess %q has expired", access.GetName())
	}
	if obj.Status == nil || !slices.ContainsFunc(obj.Status.Conditions, func(c metav1.Condition) bool {
		return c.Type == "Ready" && c.Status == metav1.ConditionTrue
	}) {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}

	if access.GetDeletionTimestamp() != nil {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	if obj.Status.RelayAccess == "" || obj.Status.TokenSecret == "" || obj.Status.DatabaseRole == "" || obj.Status.ServerName == "" || obj.Status.ServerCASecret == "" {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	return &PostgresAccessConnectionDetails{}, obj.Status.TokenSecret, nil
}

func toPostgresAccess(u *unstructured.Unstructured, teamSlug slug.Slug, environmentName string) (*PostgresAccess, error) {
	name := u.GetName()
	obj, err := kubernetes.ToConcrete[nais_io_v1.PostgresAccess](u)
	if err != nil {
		return nil, fmt.Errorf("converting PostgresAccess %q: %w", name, err)
	}
	expiresAt := obj.Spec.ExpiresAt.Time
	levelStr := obj.Spec.AccessLevel
	level := PostgresAccessLevel(strings.ToUpper(string(levelStr)))
	if !level.IsValid() {
		return nil, fmt.Errorf("invalid accessLevel %q for PostgresAccess %q", levelStr, name)
	}

	state, message := postgresAccessState(obj, expiresAt)

	relayName := ""
	if obj.Status != nil {
		relayName = obj.Status.RelayAccess
	}

	return &PostgresAccess{
		Name:               name,
		TeamSlug:           teamSlug,
		EnvironmentName:    environmentName,
		PostgresBranchName: obj.Spec.PostgresBranch,
		Username:           obj.Spec.Username,
		AccessLevel:        level,
		ExpiresAt:          expiresAt,
		State:              state,
		Message:            strPtr(message),
		RelayAccess:        strPtr(relayName),
	}, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func postgresAccessState(obj *nais_io_v1.PostgresAccess, expiresAt time.Time) (PostgresAccessState, string) {
	if !expiresAt.IsZero() && expiresAt.Before(time.Now()) {
		return PostgresAccessStateExpired, "access has expired"
	}
	if obj.Status == nil {
		return PostgresAccessStatePending, "waiting for controller"
	}
	for _, condition := range obj.Status.Conditions {
		if condition.Type != "Ready" {
			continue
		}
		if condition.Status == metav1.ConditionTrue {
			return PostgresAccessStateReady, condition.Message
		}
		if condition.Reason == "UnsupportedAccessLevel" {
			return PostgresAccessStateFailed, condition.Message
		}
		return PostgresAccessStatePending, condition.Message
	}
	return PostgresAccessStatePending, "waiting for controller"
}

func GetReadyPostgresBranch(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName, branchName string) (*PostgresBranch, error) {
	instance, err := GetPostgresBranch(ctx, teamSlug, environmentName, postgresName, branchName)
	if err != nil {
		return nil, err
	}
	if instance.State != PostgresBranchStateAvailable {
		return instance, nil
	}
	if _, err := GetPostgres(ctx, teamSlug, environmentName, instance.PostgresName); err != nil {
		return nil, fmt.Errorf("getting Postgres %q for branch %q: %w", instance.PostgresName, branchName, err)
	}
	// The pgrator reconciliation condition reflects the CNPG phase, but must be
	// corroborated with CNPG's own Ready condition before issuing access.
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}))
	if err != nil {
		return nil, err
	}
	cluster, err := client.Namespace(teamSlug.String()).Get(ctx, nais_io_v1.CNPGClusterName(nais_io_v1.PostgresBranchObjectName(postgresName, branchName)), metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		instance.State = PostgresBranchStateProgressing
		return instance, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting CNPG Cluster for PostgresBranch %q: %w", branchName, err)
	}
	conditions, found, err := unstructured.NestedSlice(cluster.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	if !found {
		instance.State = PostgresBranchStateProgressing
		return instance, nil
	}
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if ok && condition["type"] == "Ready" && condition["status"] == "True" {
			return instance, nil
		}
	}
	instance.State = PostgresBranchStateProgressing
	return instance, nil
}

func GetPostgresBranch(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName, branchName string) (*PostgresBranch, error) {
	branch, err := fromContext(ctx).postgresBranchWatcher.Get(environmentName, teamSlug.String(), nais_io_v1.PostgresBranchObjectName(postgresName, branchName))
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, &watcher.ErrorNotFound{Cluster: environmentName, Namespace: teamSlug.String(), Name: postgresName + "/" + branchName}
	}
	if err != nil {
		return nil, err
	}
	if branch.PostgresName != postgresName || branch.Name != branchName {
		return nil, &watcher.ErrorNotFound{Cluster: environmentName, Namespace: teamSlug.String(), Name: postgresName + "/" + branchName}
	}
	return branch, nil
}

// GetPostgresBranchByObjectName resolves the internal object name from PostgresAccess.
func GetPostgresBranchByObjectName(ctx context.Context, teamSlug slug.Slug, environmentName, objectName string) (*PostgresBranch, error) {
	branch, err := fromContext(ctx).postgresBranchWatcher.Get(environmentName, teamSlug.String(), objectName)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, &watcher.ErrorNotFound{Cluster: environmentName, Namespace: teamSlug.String(), Name: "Postgres branch"}
	}
	if err != nil {
		return nil, err
	}
	if nais_io_v1.PostgresBranchObjectName(branch.PostgresName, branch.Name) != objectName {
		return nil, &watcher.ErrorNotFound{Cluster: environmentName, Namespace: teamSlug.String(), Name: "Postgres branch"}
	}
	return branch, nil
}

func ListForPostgres(ctx context.Context, pg *Postgres, page *pagination.Pagination, orderBy *PostgresBranchOrder) *PostgresBranchConnection {
	all := make([]*PostgresBranch, 0)
	for _, branch := range ListAllForTeam(ctx, pg.TeamSlug, nil) {
		if branch.EnvironmentName == pg.EnvironmentName && branch.PostgresName == pg.Name {
			all = append(all, branch)
		}
	}
	if orderBy == nil {
		orderBy = &PostgresBranchOrder{Field: PostgresBranchOrderFieldName, Direction: model.OrderDirectionAsc}
	}
	return SortFilterPostgresBranch.PaginatedList(ctx, all, page, orderBy.Field, orderBy.Direction, nil)
}

func GetPostgres(ctx context.Context, teamSlug slug.Slug, environmentName, name string) (*Postgres, error) {
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgres"}))
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

	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(postgresAccessGVR()))
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(accessTTL)
	username := authz.ActorFromContext(ctx).User.Identity()
	branchObjectName := nais_io_v1.PostgresBranchObjectName(input.Postgres, input.Branch)

	// One live access per user and branch: pgrator derives a single personal database
	// role from them, so a second access could never become ready.
	existing, err := findActivePostgresAccess(ctx, client.Namespace(input.TeamSlug.String()), username, branchObjectName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		level, _, _ := unstructured.NestedString(existing.Object, "spec", "accessLevel")
		if level != input.AccessLevel.CRDValue() {
			return nil, apierror.Errorf("You already have %s access to this branch until %s. Wait for it to expire before requesting a different access level.", level, existingExpiry(existing).Format(time.RFC3339))
		}
		return &CreatePostgresAccessPayload{Name: existing.GetName(), ExpiresAt: existingExpiry(existing)}, nil
	}

	name := fmt.Sprintf("postgres-access-%s", uuid.NewString()[:8])
	res := newPostgresAccessResource(input, username, name, expiresAt)

	if _, err := client.Namespace(input.TeamSlug.String()).Create(ctx, res, metav1.CreateOptions{}); err != nil {
		return nil, err
	}

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionCreatePersonalAccess,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    input.Postgres,
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

func existingExpiry(access *unstructured.Unstructured) time.Time {
	value, _, _ := unstructured.NestedString(access.Object, "spec", "expiresAt")
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

// findActivePostgresAccess returns the user's unexpired, non-deleting access to a branch.
// The check is not atomic with creation, so two simultaneous requests can still both create one.
func findActivePostgresAccess(ctx context.Context, client dynamic.ResourceInterface, username, branchObjectName string) (*unstructured.Unstructured, error) {
	list, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing postgres accesses: %w", err)
	}
	now := time.Now()
	for i := range list.Items {
		item := &list.Items[i]
		if item.GetDeletionTimestamp() != nil || !existingExpiry(item).After(now) {
			continue
		}
		user, _, _ := unstructured.NestedString(item.Object, "spec", "username")
		branch, _, _ := unstructured.NestedString(item.Object, "spec", "postgresBranch")
		if user == username && branch == branchObjectName {
			return item, nil
		}
	}
	return nil, nil
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
		"postgresBranch": nais_io_v1.PostgresBranchObjectName(input.Postgres, input.Branch),
		"username":       username,
		"accessLevel":    input.AccessLevel.CRDValue(),
		"expiresAt":      expiresAt.Format(time.RFC3339),
	}
	return res
}

func WorkloadsForInstance(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName, branchName string) []workload.Workload {
	instance, err := GetPostgresBranch(ctx, teamSlug, environmentName, postgresName, branchName)
	if err != nil {
		return nil
	}
	postgres, err := GetPostgres(ctx, teamSlug, environmentName, instance.PostgresName)
	if err != nil || postgres.ActiveBranch == nil || *postgres.ActiveBranch != branchName {
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
