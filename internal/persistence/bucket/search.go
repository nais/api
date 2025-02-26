package bucket

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func AddSearch(client search.Client, watcher *watcher.Watcher[*Bucket]) {
	createIdent := func(env string, obj *Bucket) ident.Ident {
		return newIdent(slug.Slug(obj.GetNamespace()), env, obj.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("BUCKET", search.NewK8sSearch("BUCKET", watcher, gbi, createIdent))
}
