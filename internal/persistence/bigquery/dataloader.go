package bigquery

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, bqWatcher *watcher.Watcher[*BigQueryDataset]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(bqWatcher))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*BigQueryDataset] {
	w := watcher.Watch(mgr, &BigQueryDataset{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toBigQueryDataset(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "google.nais.io",
		Version:  "v1",
		Resource: "bigquerydatasets",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	watcher *watcher.Watcher[*BigQueryDataset]
}

func newLoaders(bqWatcher *watcher.Watcher[*BigQueryDataset]) *loaders {
	return &loaders{
		watcher: bqWatcher,
	}
}
