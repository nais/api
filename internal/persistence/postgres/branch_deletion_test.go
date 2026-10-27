package postgres

import (
	"testing"

	v1 "github.com/nais/pgrator/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBranchDeletionRequiresAnotherBranch(t *testing.T) {
	branch := func(postgres, name string) unstructured.Unstructured {
		obj := unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{"postgres": postgres, "branchName": name},
		}}
		obj.SetName(v1.PostgresBranchObjectName(postgres, name))
		return obj
	}
	other := branch("orders", "other")
	deleting := other.DeepCopy()
	deleting.SetDeletionTimestamp(new(metav1.Now()))
	invalid := other.DeepCopy()
	invalid.SetName("invalid-identity")
	for _, tt := range []struct {
		name     string
		branches []unstructured.Unstructured
		allowed  bool
	}{
		{name: "no other branch", branches: []unstructured.Unstructured{branch("orders", "preview")}},
		{name: "another branch remains", branches: []unstructured.Unstructured{branch("orders", "preview"), other}, allowed: true},
		{name: "terminating branch does not count", branches: []unstructured.Unstructured{branch("orders", "preview"), *deleting}},
		{name: "branch in another Postgres does not count", branches: []unstructured.Unstructured{branch("orders", "preview"), branch("reports", "main")}},
		{name: "invalid identity does not count", branches: []unstructured.Unstructured{branch("orders", "preview"), *invalid}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureAnotherBranchExists("orders", "preview", &unstructured.UnstructuredList{Items: tt.branches})
			if (err == nil) != tt.allowed {
				t.Errorf("deletion error = %v, want allowed = %v", err, tt.allowed)
			}
		})
	}
}
