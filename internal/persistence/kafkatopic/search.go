package kafkatopic

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

func AddSearch(client search.Client, watcher *watcher.Watcher[*KafkaTopic]) {
	createIdent := func(env string, obj *KafkaTopic) ident.Ident {
		return newIdent(slug.Slug(obj.GetNamespace()), env, obj.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	client.AddClient("KAFKA_TOPIC", search.NewK8sSearch("KAFKA_TOPIC", watcher, gbi, createIdent))
}
