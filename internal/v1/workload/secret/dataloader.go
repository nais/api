package secret

import (
	"context"

	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, secretWatcher *watcher.Watcher[*Secret], log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		secretWatcher: secretWatcher,
		log:           log,
	})
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*Secret] {
	w := watcher.Watch(mgr, &Secret{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		return toGraphSecret(o, environmentName)
	}), watcher.WithGVR(schema.GroupVersionResource{
		Version:  "v1",
		Resource: "secrets",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	secretWatcher *watcher.Watcher[*Secret]
	log           logrus.FieldLogger
}
