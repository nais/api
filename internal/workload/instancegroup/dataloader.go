package instancegroup

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, rsWatcher *watcher.Watcher[*appsv1.ReplicaSet], podWatcher *watcher.Watcher[*corev1.Pod], log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		rsWatcher:  rsWatcher,
		podWatcher: podWatcher,
		log:        log,
	})
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*appsv1.ReplicaSet] {
	w := watcher.Watch(mgr, &appsv1.ReplicaSet{}, watcher.WithTransformer(transformReplicaSet), watcher.WithInformerFilter("app"))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	rsWatcher  *watcher.Watcher[*appsv1.ReplicaSet]
	podWatcher *watcher.Watcher[*corev1.Pod]
	log        logrus.FieldLogger
}

// transformReplicaSet strips unnecessary fields from ReplicaSets but keeps
// the data we need: labels, annotations (revision), replicas, pod template
// (containers with env/envFrom/volumeMounts/image, volumes).
func transformReplicaSet(in any) (any, error) {
	rs := in.(*unstructured.Unstructured)

	// --- Extract data we want to keep ---

	// metadata.annotations - keep only deployment.kubernetes.io/revision
	annotations := rs.GetAnnotations()
	newAnnotations := map[string]string{}
	if v, ok := annotations["deployment.kubernetes.io/revision"]; ok {
		newAnnotations["deployment.kubernetes.io/revision"] = v
	}

	// metadata.labels - keep app and team
	labelsToKeep := []string{"app", "team"}
	labels := rs.GetLabels()
	for k := range labels {
		if !slices.Contains(labelsToKeep, k) {
			delete(labels, k)
		}
	}

	// spec.replicas
	replicas, _, _ := unstructured.NestedInt64(rs.Object, "spec", "replicas")

	// spec.template.spec.containers (keep name, image, env, envFrom, volumeMounts)
	containers, _, _ := unstructured.NestedSlice(rs.Object, "spec", "template", "spec", "containers")
	newContainers := make([]any, 0, len(containers))
	for _, container := range containers {
		c, ok := container.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("container is not a map[string]any")
		}
		newC := map[string]any{
			"name": c["name"],
		}
		if img, ok := c["image"]; ok {
			newC["image"] = img
		}
		if env, ok := c["env"]; ok {
			newC["env"] = env
		}
		if envFrom, ok := c["envFrom"]; ok {
			newC["envFrom"] = envFrom
		}
		if vm, ok := c["volumeMounts"]; ok {
			newC["volumeMounts"] = vm
		}
		newContainers = append(newContainers, newC)
	}

	// spec.template.spec.volumes
	volumes, _, _ := unstructured.NestedSlice(rs.Object, "spec", "template", "spec", "volumes")

	// status.replicas and status.readyReplicas
	statusReplicas, _, _ := unstructured.NestedInt64(rs.Object, "status", "replicas")
	readyReplicas, _, _ := unstructured.NestedInt64(rs.Object, "status", "readyReplicas")

	// spec.template.metadata.labels (for pod matching)
	templateLabels, _, _ := unstructured.NestedStringMap(rs.Object, "spec", "template", "metadata", "labels")

	// metadata.ownerReferences (to link to Deployment/Application)
	ownerRefs, _, _ := unstructured.NestedSlice(rs.Object, "metadata", "ownerReferences")

	// --- Remove everything ---
	fieldsToRemove := [][]string{
		{"spec"},
		{"status"},
		{"metadata", "managedFields"},
		{"metadata", "generateName"},
	}
	for _, field := range fieldsToRemove {
		unstructured.RemoveNestedField(rs.Object, field...)
	}

	// --- Add back only what we need ---
	rs.SetLabels(labels)
	rs.SetAnnotations(newAnnotations)

	_ = unstructured.SetNestedField(rs.Object, replicas, "spec", "replicas")
	_ = unstructured.SetNestedSlice(rs.Object, newContainers, "spec", "template", "spec", "containers")
	if len(volumes) > 0 {
		_ = unstructured.SetNestedSlice(rs.Object, volumes, "spec", "template", "spec", "volumes")
	}
	if len(templateLabels) > 0 {
		templateLabelsAny := make(map[string]any, len(templateLabels))
		for k, v := range templateLabels {
			templateLabelsAny[k] = v
		}
		_ = unstructured.SetNestedMap(rs.Object, templateLabelsAny, "spec", "template", "metadata", "labels")
	}
	if len(ownerRefs) > 0 {
		_ = unstructured.SetNestedSlice(rs.Object, ownerRefs, "metadata", "ownerReferences")
	}

	_ = unstructured.SetNestedField(rs.Object, statusReplicas, "status", "replicas")
	_ = unstructured.SetNestedField(rs.Object, readyReplicas, "status", "readyReplicas")

	return rs, nil
}
