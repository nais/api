package alerts

import (
	"context"

	"github.com/sirupsen/logrus"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, client PrometheusAlertsClient, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(client, log))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	log    logrus.FieldLogger
	client PrometheusAlertsClient
}

func newLoaders(client PrometheusAlertsClient, log logrus.FieldLogger) *loaders {
	return &loaders{
		client: client,
		log:    log,
	}
}
