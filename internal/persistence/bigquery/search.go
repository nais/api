package bigquery

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
)

func AddSearch(client search.Client, watcher *watcher.Watcher[*BigQueryDataset]) {
	createIdent := func(env string, obj *BigQueryDataset) ident.Ident {
		return obj.ID()
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("BIGQUERY_DATASET", search.NewK8sSearch("BIGQUERY_DATASET", watcher, gbi, createIdent))
}
