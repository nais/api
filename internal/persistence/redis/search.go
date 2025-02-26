package redis

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func AddSearch(client search.Client, watcher *watcher.Watcher[*RedisInstance]) {
	createIdent := func(env string, app *RedisInstance) ident.Ident {
		return newIdent(slug.Slug(app.GetNamespace()), env, app.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("REDIS_INSTANCE", search.NewK8sSearch("REDIS_INSTANCE", watcher, gbi, createIdent))
}
