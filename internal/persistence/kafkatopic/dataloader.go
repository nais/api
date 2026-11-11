package kafkatopic

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, watcher *watcher.Watcher[*KafkaTopic], logger logrus.FieldLogger, credentialEnvironments CredentialEnvironments) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(watcher, logger, credentialEnvironments))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*KafkaTopic] {
	w := watcher.Watch(mgr, &KafkaTopic{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toKafkaTopic(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "kafka.nais.io",
		Version:  "v1",
		Resource: "topics",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	watcher                *watcher.Watcher[*KafkaTopic]
	log                    logrus.FieldLogger
	credentialEnvironments CredentialEnvironments
}

func newLoaders(watcher *watcher.Watcher[*KafkaTopic], logger logrus.FieldLogger, credentialEnvironments CredentialEnvironments) *loaders {
	return &loaders{
		watcher:                watcher,
		log:                    logger,
		credentialEnvironments: credentialEnvironments,
	}
}
