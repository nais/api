package utilization

import (
	"context"
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
	client ResourceUsageClient
}

func newLoaders(client ResourceUsageClient) *loaders {
	return &loaders{
		client: client,
	}
}
