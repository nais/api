package loki

import (
	"context"
)

type ctxKey int

const loadersKey ctxKey = iota

type loaders struct {
	client Client
}

func NewLoaderContext(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		client: client,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
