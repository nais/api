package valkey

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, tenantName string, valkeyWatcher *watcher.Watcher[*Valkey]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(tenantName, valkeyWatcher))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*Valkey] {
	w := watcher.Watch(mgr, &Valkey{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toValkey(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "aiven.io",
		Version:  "v1alpha1",
		Resource: "valkeys",
	}), watcher.WithQuickDelete())
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	client     *client
	tenantName string
	watcher    *watcher.Watcher[*Valkey]
}

func newLoaders(tenantName string, watcher *watcher.Watcher[*Valkey]) *loaders {
	client := &client{
		watcher: watcher,
	}

	return &loaders{
		client:     client,
		tenantName: tenantName,
		watcher:    watcher,
	}
}
