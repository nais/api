package workload

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTransformPodKeepsSchedulingCondition(t *testing.T) {
	pod := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "app"},
		"spec":     map[string]any{"containers": []any{map[string]any{"name": "app", "image": "image"}}},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False"},
				map[string]any{"type": "PodScheduled", "status": "False", "reason": "Unschedulable", "message": "0/3 nodes are available", "lastTransitionTime": "2026-01-01T00:00:00Z"},
			},
		},
	}}

	out, err := transformPod(pod)
	if err != nil {
		t.Fatal(err)
	}
	conditions, _, err := unstructured.NestedSlice(out.(*unstructured.Unstructured).Object, "status", "conditions")
	if err != nil {
		t.Fatal(err)
	}
	want := []any{map[string]any{"type": "PodScheduled", "status": "False", "reason": "Unschedulable", "message": "0/3 nodes are available"}}
	if diff := cmp.Diff(want, conditions); diff != "" {
		t.Errorf("unexpected conditions (-want +got):\n%s", diff)
	}
}
