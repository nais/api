package postgres

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func AddSearchPostgres(client search.Client, watcher *watcher.Watcher[*Postgres]) {
	createIdent := func(env string, obj *Postgres) ident.Ident {
		return newPostgresIdent(slug.Slug(obj.GetNamespace()), env, obj.Name)
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetPostgresByIdent(ctx, id)
	}

	client.AddClient("POSTGRES", search.NewK8sSearch("POSTGRES", watcher, gbi, createIdent))
}
