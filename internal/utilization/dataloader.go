package utilization

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, client ResourceUsageClient, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(client, log))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	log      logrus.FieldLogger
	client   ResourceUsageClient
	location *time.Location
}

func newLoaders(client ResourceUsageClient, log logrus.FieldLogger) *loaders {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return &loaders{
		client:   client,
		location: loc,
		log:      log,
	}
}
