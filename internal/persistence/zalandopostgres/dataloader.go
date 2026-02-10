package zalandopostgres

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(
	ctx context.Context,
	zalandoPostgresWatcher *watcher.Watcher[*ZalandoPostgres],
) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(zalandoPostgresWatcher))
}

type loaders struct {
	zalandoPostgresWatcher *watcher.Watcher[*ZalandoPostgres]
}

func newLoaders(
	zalandoPostgresWatcher *watcher.Watcher[*ZalandoPostgres],
) *loaders {
	return &loaders{
		zalandoPostgresWatcher: zalandoPostgresWatcher,
	}
}

func NewZalandoPostgresWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*ZalandoPostgres] {
	w := watcher.Watch(mgr, &ZalandoPostgres{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toZalandoPostgres(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "data.nais.io",
		Version:  "v1",
		Resource: "postgres",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
