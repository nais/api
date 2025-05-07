package price

import (
	"context"

	"github.com/sirupsen/logrus"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, client Retriever, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(client, log))
}

type loaders struct {
	client Retriever
	log    logrus.FieldLogger
}

func newLoaders(client Retriever, log logrus.FieldLogger) *loaders {
	return &loaders{
		client: client,
		log:    log,
	}
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
