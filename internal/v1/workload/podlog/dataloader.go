package podlog

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type ctxKey int

const loadersKey ctxKey = iota

type loaders struct {
	clients map[string]kubernetes.Interface
	log     logrus.FieldLogger
}

func NewLoaderContext(ctx context.Context, clients map[string]kubernetes.Interface, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		clients: clients,
		log:     log,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
