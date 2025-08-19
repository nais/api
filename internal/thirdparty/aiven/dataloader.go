package aiven

import (
	"context"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, projects Projects) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(projects))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	projects Projects
}

func newLoaders(projects Projects) *loaders {
	return &loaders{
		projects: projects,
	}
}
