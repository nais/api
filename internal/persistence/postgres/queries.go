package postgres

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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

	if err = logPostgresChange(ctx, activityLogEntryActionBranchDeleted, input.Postgres, input.EnvironmentName, input.TeamSlug, PostgresBranchActivityLogEntryData{
		Branch: input.Branch,
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

// ListPostgresForWorkload resolves the Postgres databases referenced by a workload,
// including databases without an active branch.
func ListPostgresForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, uses []liberatorv1.PostgresUse) ([]*Postgres, error) {
	instances := make([]*Postgres, 0, len(uses))
	for _, use := range uses {
		pg, err := GetPostgres(ctx, teamSlug, environmentName, use.Name)
		if err != nil {
			return nil, err
		}
		instances = append(instances, pg)
	}
	slices.SortFunc(instances, func(a, b *Postgres) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return instances, nil
}

func ListPostgresForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, filter *PostgresFilter) *PostgresConnection {
	all := watcher.Objects(fromContext(ctx).postgresWatcher.GetByNamespace(teamSlug.String(), watcher.WithoutDeleted()))
	instances := make([]*Postgres, 0, len(all))
	for _, pg := range all {
		if filter.Matches(pg) {
			instances = append(instances, pg)
		}
	}
	slices.SortFunc(instances, func(a, b *Postgres) int {
		if n := strings.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})
	conn := pagination.NewConnection(pagination.Slice(instances, page), page, len(instances))
	return pagination.NewFacetableConnection(conn, all, filter)
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
	endpoint, _, err := unstructured.NestedString(access.Object, "status", "relayEndpoint")
	if err != nil {
		return nil, fmt.Errorf("reading relay endpoint for PostgresAccess %q: %w", input.Name, err)
	}
	if endpoint == "" {
		return nil, nil
	}

	connection, credentialSecretName, err := postgresAccessConnectionDetails(access, time.Now())
	if err != nil {
		return nil, err
	}

	if err := loadPostgresAccessConnection(ctx, access, input, connection, credentialSecretName); err != nil {
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
	endpoint, _, err := unstructured.NestedString(access.Object, "status", "relayEndpoint")
	if err != nil || endpoint == "" {
		return nil, "", apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	return &PostgresAccessConnectionDetails{RelayEndpoint: endpoint}, obj.Status.TokenSecret, nil
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

	// One access per user and branch: pgrator derives a single personal database role from
	// them, so a second access could never become ready. The name is derived from the pair, so
	// the API server enforces this atomically through AlreadyExists.
	name := postgresAccessName(username, branchObjectName)
	accesses := client.Namespace(input.TeamSlug.String())

	res := newPostgresAccessResource(input, username, name, expiresAt)
	for attempt := 0; ; attempt++ {
		_, err = accesses.Create(ctx, res, metav1.CreateOptions{})
		if err == nil {
			break
		}
		if !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
		existing, getErr := accesses.Get(ctx, name, metav1.GetOptions{})
		if k8serrors.IsNotFound(getErr) {
			if attempt == 0 {
				continue // Deleted between Create and Get; retry once.
			}
			return nil, apierror.Errorf("Your previous access to this branch is still being cleaned up. Try again later.")
		}
		if getErr != nil {
			return nil, getErr
		}
		if existing.GetDeletionTimestamp() == nil && existingExpiry(existing).After(time.Now()) {
			level, _, _ := unstructured.NestedString(existing.Object, "spec", "accessLevel")
			if level != input.AccessLevel.CRDValue() {
				return nil, apierror.Errorf("You already have %s access to this branch until %s. Wait for it to expire before requesting a different access level.", level, existingExpiry(existing).Format(time.RFC3339))
			}
			return &CreatePostgresAccessPayload{Name: name, ExpiresAt: existingExpiry(existing)}, nil
		}
		if attempt > 0 {
			return nil, apierror.Errorf("Your previous access to this branch is still being cleaned up. Try again later.")
		}
		// The immutable expired access must disappear before it can be replaced.
		// Only delete the UID we inspected, never a concurrent replacement.
		uid := existing.GetUID()
		if existing.GetDeletionTimestamp() == nil {
			if err := accesses.Delete(ctx, name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil && !k8serrors.IsNotFound(err) && !k8serrors.IsConflict(err) {
				return nil, err
			}
		}
		if err := waitForPostgresAccessDeletion(ctx, accesses, name, uid); err != nil {
			return nil, err
		}
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

func waitForPostgresAccessDeletion(ctx context.Context, accesses dynamic.ResourceInterface, name string, uid types.UID) error {
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := wait.PollUntilContextCancel(waitCtx, 250*time.Millisecond, true, func(ctx context.Context) (bool, error) {
		current, err := accesses.Get(ctx, name, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return current.GetUID() != uid, nil
	})
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return apierror.Errorf("Your previous access to this branch is still being cleaned up after 10 seconds (access %s). Try again later.", name)
	}
	return err
}

func existingExpiry(access *unstructured.Unstructured) time.Time {
	value, _, _ := unstructured.NestedString(access.Object, "spec", "expiresAt")
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

// postgresAccessName derives the access name from user and branch, so the Kubernetes API
// server rejects a second concurrent access for the same pair. The NUL separator keeps
// different (user, branch) pairs from producing the same input.
func postgresAccessName(username, branchObjectName string) string {
	sum := sha256.Sum256([]byte(username + "\x00" + branchObjectName))
	return "postgres-access-" + hex.EncodeToString(sum[:])[:16]
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

func postgresGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgres"}
}

// postgresClient returns a namespaced client for Postgres that writes with the API's own
// identity. Users never write the CRD directly; authorization is decided by the resolver.
func postgresClient(ctx context.Context, environmentName string, teamSlug slug.Slug) (dynamic.ResourceInterface, error) {
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(postgresGVR()))
	if err != nil {
		return nil, err
	}
	return client.Namespace(teamSlug.String()), nil
}

func Create(ctx context.Context, input CreatePostgresInput) (*CreatePostgresPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := postgresClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	pg := &nais_io_v1.Postgres{
		TypeMeta: metav1.TypeMeta{Kind: "Postgres", APIVersion: "nais.io/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.TeamSlug.String(),
		},
		Spec: nais_io_v1.PostgresSpec{MajorVersion: input.MajorVersion},
	}
	pg.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(pg)
	if input.HighAvailability != nil {
		pg.Spec.HighAvailability = *input.HighAvailability
	}
	if err := applyResources(&pg.Spec.Resources, input.CPU, input.Memory, input.DiskSize); err != nil {
		return nil, err
	}

	obj, err := toUnstructuredWithoutZeroResources(pg)
	if err != nil {
		return nil, err
	}

	created, err := client.Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, apierror.ErrAlreadyExists
		}
		return nil, err
	}

	if err := logPostgresChange(ctx, activitylog.ActivityLogEntryActionCreated, input.Name, input.EnvironmentName, input.TeamSlug, nil); err != nil {
		return nil, err
	}

	ret, err := toPostgres(created, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &CreatePostgresPayload{Postgres: ret}, nil
}

func Update(ctx context.Context, input UpdatePostgresInput) (*UpdatePostgresPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := postgresClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	existing, err := client.Get(ctx, input.Name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("Postgres %q not found.", input.Name)
	}
	if err != nil {
		return nil, err
	}

	pg, err := kubernetes.ToConcrete[nais_io_v1.Postgres](existing)
	if err != nil {
		return nil, err
	}

	var changes []*PostgresUpdatedActivityLogEntryDataUpdatedField
	for _, f := range []func(*nais_io_v1.Postgres, UpdatePostgresInput) ([]*PostgresUpdatedActivityLogEntryDataUpdatedField, error){
		updateHighAvailability,
		updateResources,
	} {
		res, err := f(pg, input)
		if err != nil {
			return nil, err
		}
		changes = append(changes, res...)
	}

	if len(changes) == 0 {
		ret, err := toPostgres(existing, input.EnvironmentName)
		if err != nil {
			return nil, err
		}
		return &UpdatePostgresPayload{Postgres: ret}, nil
	}

	obj, err := toUnstructuredWithoutZeroResources(pg)
	if err != nil {
		return nil, err
	}
	obj.SetAnnotations(kubernetes.WithCommonAnnotations(obj.GetAnnotations(), authz.ActorFromContext(ctx).User.Identity()))

	updated, err := client.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	if err := logPostgresChange(ctx, activitylog.ActivityLogEntryActionUpdated, input.Name, input.EnvironmentName, input.TeamSlug, PostgresUpdatedActivityLogEntryData{UpdatedFields: changes}); err != nil {
		return nil, err
	}

	ret, err := toPostgres(updated, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &UpdatePostgresPayload{Postgres: ret}, nil
}

// applyResources sets the quantities that were provided. Omitted values are left as they are,
// so the CRD defaults apply on create and existing values are kept on update.
func applyResources(r *nais_io_v1.PostgresResources, cpu, memory, diskSize *string) error {
	set := func(field string, value *string, target *resource.Quantity) error {
		if value == nil {
			return nil
		}
		q, err := resource.ParseQuantity(*value)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", field, err)
		}
		*target = q
		return nil
	}
	if err := set("cpu", cpu, &r.Cpu); err != nil {
		return err
	}
	if err := set("memory", memory, &r.Memory); err != nil {
		return err
	}
	return set("diskSize", diskSize, &r.DiskSize)
}

// toUnstructuredWithoutZeroResources converts pg and drops resource quantities that were never
// set. resource.Quantity is a struct, so omitempty does not skip it and it would otherwise be
// sent as "0", which stops the CRD from applying its defaults.
func toUnstructuredWithoutZeroResources(pg *nais_io_v1.Postgres) (*unstructured.Unstructured, error) {
	obj, err := kubernetes.ToUnstructured(pg)
	if err != nil {
		return nil, err
	}
	for field, q := range map[string]resource.Quantity{
		"cpu": pg.Spec.Resources.Cpu, "memory": pg.Spec.Resources.Memory, "diskSize": pg.Spec.Resources.DiskSize,
	} {
		if q.IsZero() {
			unstructured.RemoveNestedField(obj.Object, "spec", "resources", field)
		}
	}
	if m, found, _ := unstructured.NestedMap(obj.Object, "spec", "resources"); found && len(m) == 0 {
		unstructured.RemoveNestedField(obj.Object, "spec", "resources")
	}
	return obj, nil
}

func logPostgresChange(ctx context.Context, action activitylog.ActivityLogEntryAction, name, environmentName string, teamSlug slug.Slug, data any) error {
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          action,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    name,
		EnvironmentName: new(environmentName),
		TeamSlug:        new(teamSlug),
		Data:            data,
	})
}

func updateHighAvailability(pg *nais_io_v1.Postgres, input UpdatePostgresInput) ([]*PostgresUpdatedActivityLogEntryDataUpdatedField, error) {
	if input.HighAvailability == nil || pg.Spec.HighAvailability == *input.HighAvailability {
		return nil, nil
	}
	old := pg.Spec.HighAvailability
	pg.Spec.HighAvailability = *input.HighAvailability
	return []*PostgresUpdatedActivityLogEntryDataUpdatedField{{
		Field:    "highAvailability",
		OldValue: new(strconv.FormatBool(old)),
		NewValue: new(strconv.FormatBool(pg.Spec.HighAvailability)),
	}}, nil
}

// updateResources applies the quantities that were provided. Omitted values are left as they
// are, and a value that is semantically unchanged (1024Mi vs 1Gi) is not reported as a change.
func updateResources(pg *nais_io_v1.Postgres, input UpdatePostgresInput) ([]*PostgresUpdatedActivityLogEntryDataUpdatedField, error) {
	var changes []*PostgresUpdatedActivityLogEntryDataUpdatedField
	for _, f := range []struct {
		name   string
		value  *string
		target *resource.Quantity
	}{
		{"cpu", input.CPU, &pg.Spec.Resources.Cpu},
		{"memory", input.Memory, &pg.Spec.Resources.Memory},
		{"diskSize", input.DiskSize, &pg.Spec.Resources.DiskSize},
	} {
		if f.value == nil {
			continue
		}
		q, err := resource.ParseQuantity(*f.value)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f.name, err)
		}
		if f.target.Cmp(q) == 0 {
			continue
		}
		change := &PostgresUpdatedActivityLogEntryDataUpdatedField{Field: f.name, NewValue: new(q.String())}
		if !f.target.IsZero() {
			change.OldValue = new(f.target.String())
		}
		*f.target = q
		changes = append(changes, change)
	}
	return changes, nil
}
