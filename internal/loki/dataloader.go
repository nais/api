package loki

import (
	"context"
)

type ctxKey int

const loadersKey ctxKey = iota

type loaders struct {
	querier Querier
}

func NewLoaderContext(ctx context.Context, querier Querier) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		querier: querier,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
