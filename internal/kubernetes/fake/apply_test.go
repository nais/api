package fake_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/apply"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestApplyRemovesOwnedFieldsInFakeClient(t *testing.T) {
	for _, test := range []struct {
		kind     string
		gvr      schema.GroupVersionResource
		existing map[string]any
		desired  map[string]any
	}{
		{
			kind: "Application",
			gvr:  schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"},
			existing: map[string]any{"spec": map[string]any{
				"image": "example.com/app:v1", "resources": map[string]any{"limits": map[string]any{"cpu": "1", "memory": "512Mi"}},
			}},
			desired: map[string]any{"spec": map[string]any{
				"image": "example.com/app:v1", "resources": map[string]any{"limits": map[string]any{"memory": "512Mi"}},
			}},
		},
		{
			kind:     "ConfigMap",
			gvr:      schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
			existing: map[string]any{"data": map[string]any{"kept": "old", "removed": "old"}, "binaryData": map[string]any{"removed": "b2xk"}},
			desired:  map[string]any{"data": map[string]any{"kept": "new"}},
		},
		{
			kind:     "Secret",
			gvr:      schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
			existing: map[string]any{"data": map[string]any{"kept": "b2xk", "removed": "b2xk"}},
			desired:  map[string]any{"data": map[string]any{"kept": "bmV3"}},
		},
	} {
		t.Run(test.kind, func(t *testing.T) {
			scheme, err := kubernetes.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			client := fake.NewDynamicClient(scheme)
			existing := (&unstructured.Unstructured{Object: test.existing}).DeepCopy()
			existing.SetGroupVersionKind(test.gvr.GroupVersion().WithKind(test.kind))
			existing.SetName("my-resource")
			existing.SetNamespace("my-team")
			existing.SetLabels(map[string]string{"kept": "same", "removed": "old"})
			existing.SetAnnotations(map[string]string{"removed": "old"})
			if _, err := apply.ApplyResource(t.Context(), client, test.gvr, existing); err != nil {
				t.Fatal(err)
			}
			desired := (&unstructured.Unstructured{Object: test.desired}).DeepCopy()
			desired.SetGroupVersionKind(existing.GroupVersionKind())
			desired.SetName(existing.GetName())
			desired.SetNamespace(existing.GetNamespace())
			desired.SetLabels(map[string]string{"kept": "same"})

			result, err := apply.ApplyResource(t.Context(), client, test.gvr, desired)
			if err != nil {
				t.Fatal(err)
			}
			actual := result.After.DeepCopy()
			actual.SetManagedFields(nil)
			if diff := cmp.Diff(desired.Object, actual.Object); diff != "" {
				t.Errorf("fake SSA mismatch (-want +got):\n%s", diff)
			}
			reapplied, err := apply.ApplyResource(t.Context(), client, test.gvr, desired)
			if err != nil {
				t.Fatal(err)
			}
			if changes := apply.Diff(reapplied.Before, reapplied.After); len(changes) != 0 {
				t.Errorf("reapplying changed the fake resource: %+v", changes)
			}
		})
	}
}

func TestFakeClientPreservesOwnershipAndConflicts(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	client := fake.NewDynamicClient(scheme)
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	resourceClient := client.Resource(gvr).Namespace("my-team")
	application := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1alpha1", "kind": "Application",
		"metadata": map[string]any{"name": "my-app", "namespace": "my-team"},
		"spec":     map[string]any{"image": "example.com/app:v1", "resources": map[string]any{"limits": map[string]any{"cpu": "1"}}},
	}}
	if _, err := apply.ApplyResource(t.Context(), client, gvr, application); err != nil {
		t.Fatal(err)
	}
	controller := application.DeepCopy()
	controller.Object["spec"] = map[string]any{"resources": map[string]any{"limits": map[string]any{"cpu": "1"}}}
	controller.SetFinalizers([]string{"controller.example.com/cleanup"})
	data, err := json.Marshal(controller.Object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resourceClient.Patch(t.Context(), controller.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: "resource-controller",
	}); err != nil {
		t.Fatal(err)
	}
	desired := application.DeepCopy()
	unstructured.RemoveNestedField(desired.Object, "spec", "resources")
	result, err := apply.ApplyResource(t.Context(), client, gvr, desired)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(application.Object["spec"], result.After.Object["spec"]); diff != "" {
		t.Errorf("controller-owned CPU was removed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(controller.GetFinalizers(), result.After.GetFinalizers()); diff != "" {
		t.Errorf("controller finalizers changed (-want +got):\n%s", diff)
	}
	before := result.After.DeepCopy()
	conflicting := application.DeepCopy()
	if err := unstructured.SetNestedField(conflicting.Object, "2", "spec", "resources", "limits", "cpu"); err != nil {
		t.Fatal(err)
	}
	if result, err := apply.ApplyResource(t.Context(), client, gvr, conflicting); result != nil || !apierrors.IsConflict(err) {
		t.Fatalf("expected ownership conflict, got result=%+v, err=%v", result, err)
	}
	stored, err := resourceClient.Get(t.Context(), application.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(before.Object, stored.Object); diff != "" {
		t.Errorf("conflicting apply modified the resource (-want +got):\n%s", diff)
	}
}

func TestFakeClientMigratesLegacyOwnership(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	client := fake.NewDynamicClient(scheme)
	gvr := schema.GroupVersionResource{Group: "nais.io", Version: "v1alpha1", Resource: "applications"}
	application := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1alpha1", "kind": "Application",
		"metadata": map[string]any{"name": "my-app", "namespace": "my-team", "resourceVersion": "1"},
		"spec":     map[string]any{"image": "example.com/app:v1", "resources": map[string]any{"limits": map[string]any{"cpu": "1", "memory": "512Mi"}}},
	}}
	application.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager: "deployd", Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: application.GetAPIVersion(), FieldsType: "FieldsV1",
		FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:resources":{"f:limits":{"f:cpu":{}}}}}`)},
	}})
	if _, err := client.Resource(gvr).Namespace("my-team").Create(t.Context(), application, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	desired := application.DeepCopy()
	desired.SetManagedFields(nil)
	desired.SetResourceVersion("")
	unstructured.RemoveNestedField(desired.Object, "spec", "resources", "limits", "cpu")
	result, err := apply.ApplyResource(t.Context(), client, gvr, desired)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(desired.Object["spec"], result.After.Object["spec"]); diff != "" {
		t.Errorf("legacy-owned CPU was retained (-want +got):\n%s", diff)
	}
	for _, entry := range result.After.GetManagedFields() {
		if entry.Manager == "deployd" {
			t.Errorf("legacy ownership was not migrated: %+v", entry)
		}
	}
}
