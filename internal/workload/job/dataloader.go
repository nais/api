package job

import (
	"context"
	"fmt"

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
	job := in.(*unstructured.Unstructured)
	src := job.Object

	// metadata
	srcMeta, _ := src["metadata"].(map[string]any)
	newMeta := map[string]any{}
	if srcMeta != nil {
		for _, k := range []string{"name", "namespace", "uid", "resourceVersion"} {
			if v, ok := srcMeta[k]; ok {
				newMeta[k] = v
			}
		}
	}

	// metadata.labels - keep only "app" and "team"
	if labels, _, _ := unstructured.NestedStringMap(src, "metadata", "labels"); len(labels) > 0 {
		kept := map[string]any{}
		for _, k := range []string{"app", "team"} {
			if v, ok := labels[k]; ok {
				kept[k] = v
			}
		}
		if len(kept) > 0 {
			newMeta["labels"] = kept
		}
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

	newObj := map[string]any{
		"apiVersion": src["apiVersion"],
		"kind":       src["kind"],
		"metadata":   newMeta,
		"spec": map[string]any{
			"containers": newContainers,
		},
	}
	// status is read whole by JobRun resolvers - preserve it as-is.
	if status, ok := src["status"]; ok {
		newObj["status"] = status
	}
	job.Object = newObj
	return job, nil
}
