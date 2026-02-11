package postgres

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func AddSearchZalandoPostgres(client search.Client, watcher *watcher.Watcher[*PostgresInstance]) {
	createIdent := func(env string, obj *PostgresInstance) ident.Ident {
		return newZalandoPostgresIdent(slug.Slug(obj.GetNamespace()), env, obj.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetZalandoPostgresByIdent(ctx, id)
	}

	client.AddClient("ZALANDO_POSTGRES", search.NewK8sSearch("ZALANDO_POSTGRES", watcher, gbi, createIdent))
}
