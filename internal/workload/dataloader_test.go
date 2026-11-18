package workload

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTransformPodKeepsStatus(t *testing.T) {
	pod := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "app"},
		"spec":     map[string]any{"containers": []any{map[string]any{"name": "app", "image": "image"}}},
		"status": map[string]any{
			"initContainerStatuses": []any{map[string]any{"name": "init"}},
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
	initStatuses, _, err := unstructured.NestedSlice(out.(*unstructured.Unstructured).Object, "status", "initContainerStatuses")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]any{map[string]any{"name": "init"}}, initStatuses); diff != "" {
		t.Errorf("unexpected init container statuses (-want +got):\n%s", diff)
	}

	want := []any{map[string]any{"type": "PodScheduled", "status": "False", "reason": "Unschedulable", "message": "0/3 nodes are available"}}
	if diff := cmp.Diff(want, conditions); diff != "" {
		t.Errorf("unexpected conditions (-want +got):\n%s", diff)
	}
}
