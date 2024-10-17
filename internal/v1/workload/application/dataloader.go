package application

import (
	"context"

	"github.com/nais/api/internal/v1/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	netv1 "k8s.io/api/networking/v1"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, appWatcher *watcher.Watcher[*nais_io_v1alpha1.Application], ingressWatcher *watcher.Watcher[*netv1.Ingress]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(appWatcher, ingressWatcher))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*nais_io_v1alpha1.Application] {
	w := watcher.Watch(mgr, &nais_io_v1alpha1.Application{})
	w.Start(ctx)
	return w
}

func NewIngressWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*netv1.Ingress] {
	w := watcher.Watch(mgr, &netv1.Ingress{})
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	appWatcher     *watcher.Watcher[*nais_io_v1alpha1.Application]
	ingressWatcher *watcher.Watcher[*netv1.Ingress]
}

func newLoaders(appWatcher *watcher.Watcher[*nais_io_v1alpha1.Application], ingressWatcher *watcher.Watcher[*netv1.Ingress]) *loaders {
	return &loaders{
		appWatcher:     appWatcher,
		ingressWatcher: ingressWatcher,
	}
}
