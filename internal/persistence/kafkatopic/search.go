package kafkatopic

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func init() {
	search.Register("KAFKA_TOPIC", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}

func AddSearch(watcher *watcher.Watcher[*KafkaTopic]) {
	createIdent := func(env string, obj *KafkaTopic) ident.Ident {
		return newIdent(slug.Slug(obj.GetNamespace()), env, obj.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	search.RegisterBleve("KAFKA_TOPIC", search.NewK8sSearch(watcher, gbi, createIdent))
}
