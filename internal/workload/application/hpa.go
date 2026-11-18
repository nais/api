package application

import (
	"context"
	"slices"

	"github.com/nais/api/internal/kubernetes/watcher"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NewHorizontalPodAutoscalerWatcher watches the automatic scaling state of applications.
func NewHorizontalPodAutoscalerWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*autoscalingv2.HorizontalPodAutoscaler] {
	w := watcher.Watch(mgr, &autoscalingv2.HorizontalPodAutoscaler{}, watcher.WithTransformer(transformHorizontalPodAutoscaler), watcher.WithInformerFilter("app"))
	w.Start(ctx)
	return w
}

var horizontalPodAutoscalerConditions = []any{
	string(autoscalingv2.AbleToScale),
	string(autoscalingv2.ScalingActive),
	string(autoscalingv2.ScalingLimited),
}

// transformHorizontalPodAutoscaler keeps only the metadata, the instance counts and the scaling conditions.
func transformHorizontalPodAutoscaler(in any) (any, error) {
	hpa := in.(*unstructured.Unstructured)

	currentReplicas, _, _ := unstructured.NestedInt64(hpa.Object, "status", "currentReplicas")
	desiredReplicas, _, _ := unstructured.NestedInt64(hpa.Object, "status", "desiredReplicas")
	conditions, _, err := unstructured.NestedSlice(hpa.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	newConditions := []any{}
	for _, condition := range conditions {
		c, ok := condition.(map[string]any)
		if !ok || !slices.Contains(horizontalPodAutoscalerConditions, c["type"]) {
			continue
		}
		newConditions = append(newConditions, map[string]any{
			"type":    c["type"],
			"status":  c["status"],
			"reason":  c["reason"],
			"message": c["message"],
		})
	}

	labels := hpa.GetLabels()
	for k := range labels {
		if k != "app" && k != "team" {
			delete(labels, k)
		}
	}

	unstructured.RemoveNestedField(hpa.Object, "spec")
	unstructured.RemoveNestedField(hpa.Object, "status")
	unstructured.RemoveNestedField(hpa.Object, "metadata", "annotations")
	unstructured.RemoveNestedField(hpa.Object, "metadata", "managedFields")
	hpa.SetLabels(labels)
	_ = unstructured.SetNestedField(hpa.Object, currentReplicas, "status", "currentReplicas")
	_ = unstructured.SetNestedField(hpa.Object, desiredReplicas, "status", "desiredReplicas")
	_ = unstructured.SetNestedSlice(hpa.Object, newConditions, "status", "conditions")

	return hpa, nil
}
