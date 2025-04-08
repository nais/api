package utilization

import (
	"context"
	"time"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, client ResourceUsageClient) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(client))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	client   ResourceUsageClient
	location *time.Location
}

func newLoaders(client ResourceUsageClient) *loaders {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return &loaders{
		client:   client,
		location: loc,
	}
}
