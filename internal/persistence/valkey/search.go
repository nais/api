package valkey

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func AddSearch(client search.Client, watcher *watcher.Watcher[*Valkey]) {
	createIdent := func(env string, obj *Valkey) ident.Ident {
		return newIdent(slug.Slug(obj.GetNamespace()), env, obj.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("VALKEY_INSTANCE", search.NewK8sSearch("VALKEY_INSTANCE", watcher, gbi, createIdent))
}
