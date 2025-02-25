package redis

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func init() {
	search.Register("REDIS_INSTANCE", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}

func AddSearch(client search.Client, watcher *watcher.Watcher[*RedisInstance]) {
	createIdent := func(env string, app *RedisInstance) ident.Ident {
		return newIdent(slug.Slug(app.GetNamespace()), env, app.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("REDIS_INSTANCE", search.NewK8sSearch(watcher, gbi, createIdent))
}
