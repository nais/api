package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func AddSearchSQLInstance(client search.Client, watcher *watcher.Watcher[*SQLInstance]) {
	createIdent := func(env string, obj *SQLInstance) ident.Ident {
		return newIdent(slug.Slug(obj.GetNamespace()), env, obj.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("SQL_INSTANCE", search.NewK8sSearch("SQL_INSTANCE", watcher, gbi, createIdent))
}
