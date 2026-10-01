package postgres

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func AddSearchPostgresBranch(client search.Client, watcher *watcher.Watcher[*PostgresBranch]) {
	createIdent := func(env string, obj *PostgresBranch) ident.Ident {
		return newIdent(slug.Slug(obj.GetNamespace()), env, obj.PostgresName, obj.Name)
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetPostgresBranchByIdent(ctx, id)
	}

	client.AddClient("POSTGRES_BRANCH", search.NewK8sSearch("POSTGRES_BRANCH", watcher, gbi, createIdent))
}
