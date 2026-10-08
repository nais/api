package postgres

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Read current references directly, including manifests not yet translated into
// bindings.
func ensureBranchUnreferenced(ctx context.Context, input DeletePostgresBranchInput) error {
	for _, resource := range []struct {
		gvr  schema.GroupVersionResource
		kind string
	}{
		{schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}, "Application"},
		{schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "naisjobs"}, "Naisjob"},
		{schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgresbindings"}, "PostgresBinding"},
	} {
		objects, err := listPostgresDependents(ctx, input.EnvironmentName, input.TeamSlug, resource.gvr)
		if err != nil {
			return fmt.Errorf("listing %s references before deleting branch: %w", resource.kind, err)
		}
		for _, obj := range objects.Items {
			if resource.kind == "PostgresBinding" {
				postgres, _, err := unstructured.NestedString(obj.Object, "spec", "postgres")
				if err != nil {
					return err
				}
				branch, _, err := unstructured.NestedString(obj.Object, "spec", "branch")
				if err != nil {
					return err
				}
				if postgres == input.Postgres && branch == input.Branch {
					return apierror.Errorf("PostgresBranch %q is referenced by PostgresBinding %q; remove the reference before deletion", input.Branch, obj.GetName())
				}
				continue
			}
			uses, _, err := unstructured.NestedSlice(obj.Object, "spec", "uses", "postgres")
			if err != nil {
				return err
			}
			for _, use := range uses {
				ref, ok := use.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid Postgres reference in %s %q", resource.kind, obj.GetName())
				}
				if ref["name"] == input.Postgres && ref["branch"] == input.Branch {
					return apierror.Errorf("PostgresBranch %q is used by %s %q; remove its uses.postgres reference before deletion", input.Branch, resource.kind, obj.GetName())
				}
			}
		}
	}
	return nil
}
