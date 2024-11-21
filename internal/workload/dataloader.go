package workload

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	corev1 "k8s.io/api/core/v1"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, podWatcher *watcher.Watcher[*corev1.Pod]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(podWatcher))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*corev1.Pod] {
	w := watcher.Watch(mgr, &corev1.Pod{})
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
