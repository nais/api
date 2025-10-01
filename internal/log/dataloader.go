package log

import (
	"context"
)

type ctxKey int

const loadersKey ctxKey = iota

type loaders struct{}

func NewLoaderContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
