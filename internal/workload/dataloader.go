package workload

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/kubernetes/watcher"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, podWatcher *watcher.Watcher[*corev1.Pod]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(podWatcher))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*corev1.Pod] {
	w := watcher.Watch(mgr, &corev1.Pod{}, watcher.WithTransformer(transformPod), watcher.WithInformerFilter("app"))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	podWatcher *watcher.Watcher[*corev1.Pod]
}

func newLoaders(podWatcher *watcher.Watcher[*corev1.Pod]) *loaders {
	return &loaders{
		podWatcher: podWatcher,
	}
}

func transformPod(in any) (any, error) {
	pod := in.(*unstructured.Unstructured)
	src := pod.Object

	// metadata
	srcMeta, _ := src["metadata"].(map[string]any)
	newMeta := map[string]any{}
	if srcMeta != nil {
		for _, k := range []string{"name", "namespace", "uid", "resourceVersion", "creationTimestamp"} {
			if v, ok := srcMeta[k]; ok {
				newMeta[k] = v
			}
		}
	}

	// metadata.labels - keep only "app", "team", "job-name"
	if labels, _, _ := unstructured.NestedStringMap(src, "metadata", "labels"); len(labels) > 0 {
		kept := map[string]any{}
		for _, k := range []string{"app", "team", "job-name"} {
			if v, ok := labels[k]; ok {
				kept[k] = v
			}
		}
		if len(kept) > 0 {
			newMeta["labels"] = kept
		}
	}

	// metadata.ownerReferences - kept verbatim (links pods to their ReplicaSet
	// for instance group matching). NestedSlice already deep-copies.
	if ownerRefs, _, _ := unstructured.NestedSlice(src, "metadata", "ownerReferences"); len(ownerRefs) > 0 {
		newMeta["ownerReferences"] = ownerRefs
	}

	// spec.containers - keep name + image only
	containers, _, err := unstructured.NestedSlice(src, "spec", "containers")
	if err != nil {
		return nil, err
	}
	newContainers := make([]any, 0, len(containers))
	for _, container := range containers {
		c, ok := container.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("container is not a map[string]any")
		}
		newC := map[string]any{}
		if v, ok := c["name"]; ok {
			newC["name"] = v
		}
		if v, ok := c["image"]; ok {
			newC["image"] = v
		}
		newContainers = append(newContainers, newC)
	}

	// status.containerStatuses - kept verbatim. NestedSlice already deep-copies.
	containerStatuses, _, err := unstructured.NestedSlice(src, "status", "containerStatuses")
	if err != nil {
		return nil, err
	}

	newSpec := map[string]any{
		"containers": newContainers,
	}

	newStatus := map[string]any{}
	if len(containerStatuses) > 0 {
		newStatus["containerStatuses"] = containerStatuses
	}

	newObj := map[string]any{
		"apiVersion": src["apiVersion"],
		"kind":       src["kind"],
		"metadata":   newMeta,
		"spec":       newSpec,
		"status":     newStatus,
	}
	pod.Object = newObj
	return pod, nil
}
