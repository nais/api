package feedback

import (
	"context"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		feedbackClient: client,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	feedbackClient Client
}
