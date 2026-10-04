package postgres

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func branchClient(ctx context.Context, environment string, team slug.Slug) (dynamic.ResourceInterface, error) {
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}
	return client.Namespace(team.String()), nil
}

func getBranchResource(ctx context.Context, client dynamic.ResourceInterface, postgres, branch string) (*unstructured.Unstructured, error) {
	obj, err := client.Get(ctx, nais_io_v1.PostgresBranchObjectName(postgres, branch), metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("PostgresBranch %q not found in Postgres %q", branch, postgres)
	}
	if err != nil {
		return nil, err
	}
	parsed, err := toPostgresBranch(obj, "")
	if err != nil || parsed.PostgresName != postgres || parsed.Name != branch || obj.GetDeletionTimestamp() != nil {
		return nil, apierror.Errorf("PostgresBranch %q not found in Postgres %q", branch, postgres)
	}
	return obj, nil
}

func CreateBranch(ctx context.Context, input CreatePostgresBranchInput) (*CreatePostgresBranchPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}
	pg, err := postgresClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}
	if _, err := pg.Get(ctx, input.Postgres, metav1.GetOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, apierror.Errorf("Postgres %q not found", input.Postgres)
		}
		return nil, err
	}
	branches, err := branchClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}
	if _, err := getBranchResource(ctx, branches, input.Postgres, input.SourceBranch); err != nil {
		return nil, err
	}
	resource := &nais_io_v1.PostgresBranch{
		TypeMeta:   metav1.TypeMeta{Kind: "PostgresBranch", APIVersion: "nais.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: nais_io_v1.PostgresBranchObjectName(input.Postgres, input.Branch), Namespace: input.TeamSlug.String()},
		Spec: nais_io_v1.PostgresBranchSpec{
			Postgres: input.Postgres, BranchName: input.Branch,
			Bootstrap: &nais_io_v1.PostgresBranchBootstrap{Recovery: &nais_io_v1.PostgresBranchRecovery{
				SourceBranch: input.SourceBranch, TargetTime: metav1.NewTime(input.TargetTime.UTC()),
			}},
		},
	}
	resource.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(resource)
	obj, err := kubernetes.ToUnstructured(resource)
	if err != nil {
		return nil, err
	}
	created, err := branches.Create(ctx, obj, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		created, err = getBranchResource(ctx, branches, input.Postgres, input.Branch)
		if err != nil {
			return nil, err
		}
		if !sameRecovery(created, input) {
			return nil, apierror.Errorf("PostgresBranch %q already exists with different recovery settings", input.Branch)
		}
	} else if err != nil {
		return nil, err
	} else if err := logPostgresChange(ctx, activitylog.ActivityLogEntryActionUpdated, input.Postgres, input.EnvironmentName, input.TeamSlug, PostgresUpdatedActivityLogEntryData{
		UpdatedFields: []*PostgresUpdatedActivityLogEntryDataUpdatedField{{
			Field: "branch/" + input.Branch, NewValue: new(fmt.Sprintf("recovered from %s at %s", input.SourceBranch, input.TargetTime.Format("2006-01-02T15:04:05Z"))),
		}},
	}); err != nil {
		return nil, err
	}
	branch, err := toPostgresBranch(created, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &CreatePostgresBranchPayload{PostgresBranch: branch}, nil
}

func sameRecovery(obj *unstructured.Unstructured, input CreatePostgresBranchInput) bool {
	branch, err := kubernetes.ToConcrete[nais_io_v1.PostgresBranch](obj)
	return err == nil && branch.Spec.Postgres == input.Postgres && branch.Spec.BranchName == input.Branch &&
		branch.Spec.Bootstrap != nil && branch.Spec.Bootstrap.Recovery != nil &&
		branch.Spec.Bootstrap.Recovery.SourceBranch == input.SourceBranch &&
		branch.Spec.Bootstrap.Recovery.TargetTime.Time.Equal(input.TargetTime)
}

func ActivateBranch(ctx context.Context, input ActivatePostgresBranchInput) (*ActivatePostgresBranchPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}
	branches, err := branchClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}
	branch, err := getBranchResource(ctx, branches, input.Postgres, input.Branch)
	if err != nil {
		return nil, err
	}
	state, err := toPostgresBranch(branch, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	if state.State != PostgresBranchStateAvailable {
		return nil, apierror.Errorf("PostgresBranch %q is not available", input.Branch)
	}
	// Verify the backing cluster independently of pgrator's reported phase.
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}))
	if err != nil {
		return nil, err
	}
	cluster, err := client.Namespace(input.TeamSlug.String()).Get(ctx, nais_io_v1.CNPGClusterName(branch.GetName()), metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("PostgresBranch %q is not ready", input.Branch)
	}
	if err != nil {
		return nil, fmt.Errorf("getting CNPG Cluster for PostgresBranch %q: %w", input.Branch, err)
	}
	conditions, _, err := unstructured.NestedSlice(cluster.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	ready := false
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if ok && condition["type"] == "Ready" && condition["status"] == "True" {
			ready = true
		}
	}
	if !ready {
		return nil, apierror.Errorf("PostgresBranch %q is not ready", input.Branch)
	}
	pgClient, err := postgresClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}
	// A Kubernetes update includes resourceVersion, so concurrent activation requests
	// cannot silently overwrite one another. Do not retry a conflict with stale intent.
	existing, err := pgClient.Get(ctx, input.Postgres, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("Postgres %q not found", input.Postgres)
	}
	if err != nil {
		return nil, err
	}
	requested, _, err := unstructured.NestedString(existing.Object, "spec", "activeBranch")
	if err != nil {
		return nil, err
	}
	if requested == input.Branch {
		result, err := toPostgres(existing, input.EnvironmentName)
		return &ActivatePostgresBranchPayload{Postgres: result}, err
	}
	// Preserve fields added by newer Postgres CRDs; update only the requested selection.
	obj := existing.DeepCopy()
	if err := unstructured.SetNestedField(obj.Object, input.Branch, "spec", "activeBranch"); err != nil {
		return nil, err
	}
	obj.SetAnnotations(kubernetes.WithCommonAnnotations(obj.GetAnnotations(), authz.ActorFromContext(ctx).User.Identity()))
	updated, err := pgClient.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if err := logPostgresChange(ctx, activitylog.ActivityLogEntryActionUpdated, input.Postgres, input.EnvironmentName, input.TeamSlug, PostgresUpdatedActivityLogEntryData{
		UpdatedFields: []*PostgresUpdatedActivityLogEntryDataUpdatedField{{
			Field: "activeBranch", OldValue: strPtr(requested), NewValue: new(input.Branch),
		}},
	}); err != nil {
		return nil, err
	}
	result, err := toPostgres(updated, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &ActivatePostgresBranchPayload{Postgres: result}, nil
}
