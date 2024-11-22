package workload

import (
	"context"
	"fmt"
	"slices"

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
	fieldsToRemove := [][]string{
		{"spec"},
		{"status"},
		{"metadata", "generateName"},
		{"metadata", "ownerReferences"},
		{"metadata", "annotations"},
		{"metadata", "managedFields"},
		{"status", "initContainerStatuses"},
	}

	labelsToKeep := []string{
		"app",
		"team",
	}

	pod := in.(*unstructured.Unstructured)

	// Getting data to keep
	containers, _, err := unstructured.NestedSlice(pod.Object, "spec", "containers")
	if err != nil {
		return nil, err
	}

	newContainers := []any{}
	for _, container := range containers {
		c, ok := container.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("container is not a map[string]any")
		}
		img, _, err := unstructured.NestedString(c, "image")
		if err != nil {
			return nil, err
		}
		newContainers = append(newContainers, map[string]any{
			"name":  c["name"],
			"image": img,
		})
	}

	containerStatuses, _, err := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
	if err != nil {
		return nil, err
	}

	// Removing data
	for _, field := range fieldsToRemove {
		unstructured.RemoveNestedField(pod.Object, field...)
	}

	labels := pod.GetLabels()
	for k := range labels {
		if !slices.Contains(labelsToKeep, k) {
			delete(labels, k)
		}
	}
	pod.SetLabels(labels)

	// Adding data back
	unstructured.SetNestedSlice(pod.Object, newContainers, "spec", "containers")
	unstructured.SetNestedSlice(pod.Object, containerStatuses, "status", "containerStatuses")

	return pod, nil
}
