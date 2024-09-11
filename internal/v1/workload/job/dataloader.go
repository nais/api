package job

import (
	"context"

	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/liberator/pkg/apis/nais.io/v1"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, jobWatcher *watcher.Watcher[*nais_io_v1.Naisjob]) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		jobWatcher: jobWatcher,
	})
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*nais_io_v1.Naisjob] {
	w := watcher.Watch(mgr, &nais_io_v1.Naisjob{})
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	jobWatcher *watcher.Watcher[*nais_io_v1.Naisjob]
}
