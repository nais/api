package deployment

import (
	"context"

	"github.com/nais/api/internal/thirdparty/hookd"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, client hookd.Client) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(client))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	client hookd.Client
}

func newLoaders(client hookd.Client) *loaders {
	return &loaders{
		client: client,
	}
}
