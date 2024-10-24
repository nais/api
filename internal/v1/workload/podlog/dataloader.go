package podlog

import (
	"context"
)

type ctxKey int

const loadersKey ctxKey = iota

type loaders struct {
	streamer Streamer
}

func NewLoaderContext(ctx context.Context, streamer Streamer) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		streamer: streamer,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
