package postgres

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DeletePostgres requests deletion only. Pgrator's finalizer performs the actual cleanup.
func DeletePostgres(ctx context.Context, input DeletePostgresInput) (*DeletePostgresPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}
	client, err := postgresClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}
	pg, err := client.Get(ctx, input.Name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("Postgres %q not found", input.Name)
	}
	if err != nil {
		return nil, fmt.Errorf("getting Postgres %q: %w", input.Name, err)
	}
	if pg.GetDeletionTimestamp() != nil {
		return nil, apierror.Errorf("Postgres %q is already being deleted", input.Name)
	}
	if !hasPostgresDeletionFinalizer(pg.GetFinalizers()) {
		return nil, apierror.Errorf("Postgres %q is not yet managed by pgrator; retry when its deletion finalizer is present", input.Name)
	}

	// Read directly from the API server, not informer caches. If any list fails,
	// deletion is refused rather than assuming that there are no references.
	for _, resource := range []struct {
		gvr  schema.GroupVersionResource
		kind string
	}{
		{schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}, "Application"},
		{schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "naisjobs"}, "Naisjob"},
	} {
		objects, err := listPostgresDependents(ctx, input.EnvironmentName, input.TeamSlug, resource.gvr)
		if err != nil {
			return nil, fmt.Errorf("listing %s resources before deleting Postgres %q: %w", resource.kind, input.Name, err)
		}
		for _, obj := range objects.Items {
			uses, _, err := unstructured.NestedSlice(obj.Object, "spec", "uses", "postgres")
			if err != nil {
				return nil, fmt.Errorf("reading %s %q postgres uses: %w", resource.kind, obj.GetName(), err)
			}
			for _, use := range uses {
				ref, ok := use.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid postgres use in %s %q", resource.kind, obj.GetName())
				}
				if ref["name"] == input.Name {
					return nil, apierror.Errorf("Postgres %q is used by %s %q; remove its uses.postgres reference before deletion", input.Name, resource.kind, obj.GetName())
				}
			}
		}
	}
	bindings, err := listPostgresDependents(ctx, input.EnvironmentName, input.TeamSlug, schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgresbindings"})
	if err != nil {
		return nil, fmt.Errorf("listing PostgresBindings before deleting Postgres %q: %w", input.Name, err)
	}
	for _, binding := range bindings.Items {
		name, _, err := unstructured.NestedString(binding.Object, "spec", "postgres")
		if err != nil {
			return nil, fmt.Errorf("reading PostgresBinding %q: %w", binding.GetName(), err)
		}
		if name == input.Name {
			return nil, apierror.Errorf("Postgres %q is referenced by PostgresBinding %q; wait for the binding to be removed before deletion", input.Name, binding.GetName())
		}
	}
	uid := pg.GetUID()
	if err := client.Delete(ctx, input.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil {
		return nil, fmt.Errorf("requesting deletion of Postgres %q: %w", input.Name, err)
	}
	if err := logPostgresChange(ctx, activityLogEntryActionDeletionRequested, input.Name, input.EnvironmentName, input.TeamSlug, nil); err != nil {
		return nil, err
	}
	return &DeletePostgresPayload{DeletionRequested: true}, nil
}

func hasPostgresDeletionFinalizer(finalizers []string) bool {
	return slices.Contains(finalizers, "postgres.nais.io")
}

func listPostgresDependents(ctx context.Context, environment string, team slug.Slug, gvr schema.GroupVersionResource) (*unstructured.UnstructuredList, error) {
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, environment, watcher.WithImpersonatedClientGVR(gvr))
	if err != nil {
		return nil, err
	}
	return client.Namespace(team.String()).List(ctx, metav1.ListOptions{})
}
