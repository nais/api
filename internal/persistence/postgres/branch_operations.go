package postgres

import (
	"context"
	"fmt"
	"time"

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

var (
	cnpgClusterGVR = schema.GroupVersionResource{
		Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters",
	}
	barmanObjectStoreGVR = schema.GroupVersionResource{
		Group: "barmancloud.cnpg.io", Version: "v1", Resource: "objectstores",
	}
)

func branchClient(ctx context.Context, environment string, team slug.Slug) (dynamic.ResourceInterface, error) {
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}
	return client.Namespace(team.String()), nil
}

func EarliestRestoreTime(ctx context.Context, branch *PostgresBranch) (*time.Time, error) {
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(
		ctx, branch.EnvironmentName, watcher.WithImpersonatedClientGVR(cnpgClusterGVR),
	)
	if err != nil {
		return nil, err
	}
	clusterName := nais_io_v1.CNPGClusterName(nais_io_v1.PostgresBranchObjectName(branch.PostgresName, branch.Name))
	cluster, err := client.Namespace(branch.TeamSlug.String()).Get(ctx, clusterName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting CNPG cluster %q for Postgres branch %q: %w", clusterName, branch.Name, err)
	}

	objectStoreName, err := barmanObjectStoreName(cluster)
	if err != nil {
		return nil, fmt.Errorf("reading Barman ObjectStore name from CNPG cluster %q: %w", clusterName, err)
	}
	if objectStoreName != "" {
		objectStoreClient, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(
			ctx, branch.EnvironmentName, watcher.WithImpersonatedClientGVR(barmanObjectStoreGVR),
		)
		if err != nil {
			return nil, err
		}
		objectStore, err := objectStoreClient.Namespace(branch.TeamSlug.String()).Get(ctx, objectStoreName, metav1.GetOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("getting Barman ObjectStore %q for CNPG cluster %q: %w", objectStoreName, clusterName, err)
		}
		if err == nil {
			earliest, err := firstRecoverabilityPoint(objectStore, clusterName)
			if err != nil || earliest != nil {
				return earliest, err
			}
		}
	}

	// Older CNPG configurations publish the recovery point directly on Cluster.
	value, found, err := unstructured.NestedString(cluster.Object, "status", "firstRecoverabilityPoint")
	if err != nil {
		return nil, fmt.Errorf("reading firstRecoverabilityPoint from CNPG cluster %q: %w", clusterName, err)
	}
	if !found || value == "" {
		return nil, nil
	}
	earliest, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parsing firstRecoverabilityPoint %q from CNPG cluster %q: %w", value, clusterName, err)
	}
	return &earliest, nil
}

func barmanObjectStoreName(cluster *unstructured.Unstructured) (string, error) {
	plugins, found, err := unstructured.NestedSlice(cluster.Object, "spec", "plugins")
	if err != nil || !found {
		return "", err
	}
	for _, item := range plugins {
		plugin, ok := item.(map[string]any)
		if !ok {
			return "", fmt.Errorf("CNPG plugin configuration has unexpected type %T", item)
		}
		name, found, err := unstructured.NestedString(plugin, "parameters", "barmanObjectName")
		if err != nil {
			return "", err
		}
		if found && name != "" {
			return name, nil
		}
	}
	return "", nil
}

func firstRecoverabilityPoint(objectStore *unstructured.Unstructured, clusterName string) (*time.Time, error) {
	value, found, err := unstructured.NestedString(objectStore.Object, "status", "serverRecoveryWindow", clusterName, "firstRecoverabilityPoint")
	if err != nil {
		return nil, err
	}
	if !found || value == "" {
		return nil, nil
	}
	earliest, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parsing firstRecoverabilityPoint %q: %w", value, err)
	}
	return &earliest, nil
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
	parent, err := pg.Get(ctx, input.Postgres, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("Postgres %q not found", input.Postgres)
	}
	if err != nil {
		return nil, err
	}
	if parent.GetDeletionTimestamp() != nil {
		return nil, apierror.Errorf("Postgres %q is being deleted", input.Postgres)
	}
	branches, err := branchClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}
	existing, err := branches.Get(ctx, nais_io_v1.PostgresBranchObjectName(input.Postgres, input.Branch), metav1.GetOptions{})
	if err == nil {
		if existing.GetDeletionTimestamp() != nil {
			return nil, apierror.Errorf("PostgresBranch %q is being deleted", input.Branch)
		}
		if !sameRecovery(existing, input) {
			return nil, apierror.Errorf("PostgresBranch %q already exists with different recovery settings", input.Branch)
		}
		branch, err := toPostgresBranch(existing, input.EnvironmentName)
		if err != nil {
			return nil, err
		}
		return &CreatePostgresBranchPayload{PostgresBranch: branch}, nil
	}
	if !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("checking whether PostgresBranch %q already exists: %w", input.Branch, err)
	}
	if _, err := getBranchResource(ctx, branches, input.Postgres, input.SourceBranch); err != nil {
		return nil, err
	}
	source := &PostgresBranch{
		Name: input.SourceBranch, EnvironmentName: input.EnvironmentName,
		TeamSlug: input.TeamSlug, PostgresName: input.Postgres,
	}
	earliest, err := EarliestRestoreTime(ctx, source)
	if err != nil {
		return nil, err
	}
	if earliest == nil {
		return nil, apierror.Errorf("Postgres branch %q has no recoverable restore window yet", input.SourceBranch)
	}
	if input.TargetTime.Before(*earliest) {
		return nil, apierror.Errorf("restore time must not be before the earliest recoverable time %s", earliest.UTC().Format(time.RFC3339))
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
	} else if err := logPostgresChange(ctx, activityLogEntryActionBranchCreated, input.Postgres, input.EnvironmentName, input.TeamSlug, PostgresBranchActivityLogEntryData{
		Branch: input.Branch, SourceBranch: new(input.SourceBranch), TargetTime: new(input.TargetTime.UTC()),
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
	if _, err := getBranchResource(ctx, branches, input.Postgres, input.Branch); err != nil {
		return nil, err
	}
	// Pgrator observes cluster readiness before switching the active branch.
	// This mutation only records the requested branch.
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
	if err := logPostgresChange(ctx, activityLogEntryActionBranchActivated, input.Postgres, input.EnvironmentName, input.TeamSlug, PostgresBranchActivityLogEntryData{
		Branch: input.Branch,
	}); err != nil {
		return nil, err
	}
	result, err := toPostgres(updated, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &ActivatePostgresBranchPayload{Postgres: result}, nil
}
