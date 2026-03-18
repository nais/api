package configmap

import (
	"context"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, watcher *watcher.Watcher[*Config], log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		watcher: watcher,
		log:     log,
	})
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*Config] {
	w := watcher.Watch(
		mgr,
		&Config{},
		watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (any, bool) {
			return toGraphConfig(o, environmentName)
		}),
		watcher.WithInformerFilter(kubernetes.IsManagedByConsoleLabelSelector()),
		watcher.WithGVR(corev1.SchemeGroupVersion.WithResource("configmaps")),
	)
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	watcher *watcher.Watcher[*Config]
	log     logrus.FieldLogger
}

// Watcher returns the configmap watcher
func (l *loaders) Watcher() *watcher.Watcher[*Config] {
	return l.watcher
}
