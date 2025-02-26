package application

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
)

func AddSearch(client search.Client, watcher *watcher.Watcher[*nais_io_v1alpha1.Application]) {
	createIdent := func(env string, obj *nais_io_v1alpha1.Application) ident.Ident {
		return newIdent(slug.Slug(obj.GetNamespace()), env, obj.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("APPLICATION", search.NewK8sSearch("APPLICATION", watcher, gbi, createIdent))
}
