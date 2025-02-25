package search

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
)

type K8sSearch[T watcher.Object] struct {
	watcher    *watcher.Watcher[T]
	getByIdent func(ctx context.Context, id ident.Ident) (SearchNode, error)

	newIdent func(env string, o T) ident.Ident
}

func NewK8sSearch[T watcher.Object](
	watcher *watcher.Watcher[T],
	getByIdent func(ctx context.Context, id ident.Ident) (SearchNode, error),
	newIdent func(env string, o T) ident.Ident,
) *K8sSearch[T] {
	return &K8sSearch[T]{
		watcher:    watcher,
		getByIdent: getByIdent,
		newIdent:   newIdent,
	}
}

func (k K8sSearch[T]) Convert(ctx context.Context, ids ...ident.Ident) ([]SearchNode, error) {
	ret := make([]SearchNode, 0, len(ids))
	for _, id := range ids {
		o, err := k.getByIdent(ctx, id)
		if err != nil {
			return nil, err
		}

		ret = append(ret, o)
	}
	return ret, nil
}

func (k K8sSearch[T]) ReIndex(ctx context.Context) []Document {
	apps := k.watcher.All()
	docs := make([]Document, 0, len(apps))
	for _, app := range apps {
		team := slug.Slug(app.GetNamespace())

		docs = append(docs, Document{
			ID:   k.newIdent(app.Cluster, app.Obj).String(),
			Name: app.GetName(),
			Team: team.String(),
		})
	}

	return docs
}

func (k K8sSearch[T]) Watch(ctx context.Context, indexer Indexer) error {
	k.watcher.OnUpdate(k.onUpdate(indexer))
	k.watcher.OnRemove(k.onRemove(indexer))

	return nil
}

func (k K8sSearch[T]) onUpdate(indexer Indexer) func(string, T) {
	return func(env string, obj T) {
		team := slug.Slug(obj.GetNamespace())
		indexer.Update(Document{
			ID:   k.newIdent(env, obj).String(),
			Name: obj.GetName(),
			Team: team.String(),
		})
	}
}

func (k K8sSearch[T]) onRemove(indexer Indexer) func(string, T) {
	return func(env string, obj T) {
		indexer.Remove(k.newIdent(env, obj))
	}
}
