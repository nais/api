package alerts

import (
	"context"

	"github.com/sirupsen/logrus"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, client AlertsClient, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(client, log))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	log    logrus.FieldLogger
	client AlertsClient
}

func newLoaders(client AlertsClient, log logrus.FieldLogger) *loaders {
	return &loaders{
		client: client,
		log:    log,
	}
}
