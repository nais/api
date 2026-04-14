package tunnel

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
)

type contextKey string

const (
	loadersKey contextKey = "tunnel_loaders"
)

func FromContext(ctx context.Context) *Loaders {
	val := ctx.Value(loadersKey)
	if val == nil {
		return nil
	}
	return val.(*Loaders)
}

func WithLoaders(ctx context.Context, loaders *Loaders) context.Context {
	return context.WithValue(ctx, loadersKey, loaders)
}

type Loaders struct {
	tunnelWatcher *watcher.Watcher[*Tunnel]
}

func NewLoaderContext(ctx context.Context, tunnelWatcher *watcher.Watcher[*Tunnel]) context.Context {
	return context.WithValue(ctx, loadersKey, &Loaders{
		tunnelWatcher: tunnelWatcher,
	})
}

func NewLoaders(tunnelWatcher *watcher.Watcher[*Tunnel]) *Loaders {
	return &Loaders{
		tunnelWatcher: tunnelWatcher,
	}
}
