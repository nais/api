package fake_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/apply"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestApplyReplacesResourcesInFakeClient(t *testing.T) {
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
			existing.SetFinalizers([]string{"controller.example.com/cleanup"})
			existing.SetResourceVersion("42")
			existing.SetManagedFields([]metav1.ManagedFieldsEntry{{
				Manager: "resource-controller", Operation: metav1.ManagedFieldsOperationApply,
				APIVersion: existing.GetAPIVersion(), FieldsType: "FieldsV1",
				FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:finalizers":{}}}`)},
			}})
			if _, err := client.Resource(test.gvr).Namespace(existing.GetNamespace()).Create(t.Context(), existing, metav1.CreateOptions{}); err != nil {
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
			actual.SetResourceVersion("")
			if diff := cmp.Diff(desired.Object, actual.Object); diff != "" {
				t.Errorf("replacement mismatch (-want +got):\n%s", diff)
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
