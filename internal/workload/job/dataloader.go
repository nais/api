package job

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, jobWatcher *watcher.Watcher[*nais_io_v1.Naisjob], runWatcher *watcher.Watcher[*batchv1.Job]) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		jobWatcher: jobWatcher,
		runWatcher: runWatcher,
	})
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*nais_io_v1.Naisjob] {
	w := watcher.Watch(mgr, &nais_io_v1.Naisjob{})
	w.Start(ctx)
	return w
}

func NewRunWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*batchv1.Job] {
	w := watcher.Watch(mgr, &batchv1.Job{}, watcher.WithTransformer(transformJob))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	jobWatcher *watcher.Watcher[*nais_io_v1.Naisjob]
	runWatcher *watcher.Watcher[*batchv1.Job]
}

func transformJob(in any) (any, error) {
	fieldsToRemove := [][]string{
		{"spec"},
		{"metadata", "creationTimestamp"},
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

	job := in.(*unstructured.Unstructured)

	// Getting data to keep
	containers, _, err := unstructured.NestedSlice(job.Object, "spec", "containers")
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

	// Removing fields
	for _, path := range fieldsToRemove {
		unstructured.RemoveNestedField(job.Object, path...)
	}

	labels := job.GetLabels()
	for k := range labels {
		if !slices.Contains(labelsToKeep, k) {
			delete(labels, k)
		}
	}
	job.SetLabels(labels)

	// Adding data back
	unstructured.SetNestedSlice(job.Object, newContainers, "spec", "containers")

	return job, nil
}
