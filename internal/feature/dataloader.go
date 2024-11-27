package feature

import (
	"context"
)

type ctxKey int

const (
	loadersKey ctxKey = iota
)

func NewLoaderContext(ctx context.Context, unleash bool) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(&Features{
		Unleash: unleash,
	}))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	features *Features
}

func newLoaders(features *Features) *loaders {
	return &loaders{
		features: features,
	}
}
