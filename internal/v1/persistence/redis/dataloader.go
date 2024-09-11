package redis

import (
	"context"

	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, redisWatcher *watcher.Watcher[*RedisInstance]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(redisWatcher))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*RedisInstance] {
	w := watcher.Watch(mgr, &RedisInstance{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		if o.GetKind() != "Redis" {
			return nil, false
		}
		ret, err := toRedisInstance(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "aiven.io",
		Version:  "v1alpha1",
		Resource: "redis",
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

func newLoaders(watcher *watcher.Watcher[*RedisInstance]) *loaders {
	client := &client{
		watcher: watcher,
	}

	return &loaders{
		client: client,
	}
}
