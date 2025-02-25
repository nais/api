package bigquery

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
)

func init() {
	search.Register("BIGQUERY_DATASET", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}

func AddSearch(client search.Client, watcher *watcher.Watcher[*BigQueryDataset]) {
	createIdent := func(env string, obj *BigQueryDataset) ident.Ident {
		return obj.ID()
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("BIGQUERY_DATASET", search.NewK8sSearch(watcher, gbi, createIdent))
}
