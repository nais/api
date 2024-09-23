package opensearch

import (
	"context"

	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, watcher *watcher.Watcher[*OpenSearch]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(watcher))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*OpenSearch] {
	w := watcher.Watch(mgr, &OpenSearch{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		if o.GetKind() != "OpenSearch" {
			return nil, false
		}
		ret, err := toOpenSearch(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "aiven.io",
		Version:  "v1alpha1",
		Resource: "opensearches",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	client *client
}

func newLoaders(watcher *watcher.Watcher[*OpenSearch]) *loaders {
	client := &client{
		watcher: watcher,
	}

	return &loaders{
		client: client,
	}
}
