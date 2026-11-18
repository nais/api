package application

import (
	"context"
	"slices"

	"github.com/nais/api/internal/kubernetes/watcher"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NewDeploymentWatcher watches the rollout state of applications.
func NewDeploymentWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*appsv1.Deployment] {
	w := watcher.Watch(mgr, &appsv1.Deployment{}, watcher.WithTransformer(transformDeployment), watcher.WithInformerFilter("app"))
	w.Start(ctx)
	return w
}

// transformDeployment keeps only the metadata and the rollout conditions of deployments.
func transformDeployment(in any) (any, error) {
	deployment := in.(*unstructured.Unstructured)

	conditions, _, err := unstructured.NestedSlice(deployment.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	newConditions := []any{}
	for _, condition := range conditions {
		c, ok := condition.(map[string]any)
		if !ok || !slices.Contains([]any{string(appsv1.DeploymentProgressing), string(appsv1.DeploymentReplicaFailure)}, c["type"]) {
			continue
		}
		newConditions = append(newConditions, map[string]any{
			"type":    c["type"],
			"status":  c["status"],
			"reason":  c["reason"],
			"message": c["message"],
		})
	}

	labels := deployment.GetLabels()
	for k := range labels {
		if k != "app" && k != "team" {
			delete(labels, k)
		}
	}

	unstructured.RemoveNestedField(deployment.Object, "spec")
	unstructured.RemoveNestedField(deployment.Object, "status")
	unstructured.RemoveNestedField(deployment.Object, "metadata", "annotations")
	unstructured.RemoveNestedField(deployment.Object, "metadata", "managedFields")
	deployment.SetLabels(labels)
	_ = unstructured.SetNestedSlice(deployment.Object, newConditions, "status", "conditions")

	return deployment, nil
}
