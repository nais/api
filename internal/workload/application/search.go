package application

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
)

func init() {
	search.Register("APPLICATION", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}

func AddSearch(watcher *watcher.Watcher[*nais_io_v1alpha1.Application]) {
	createIdent := func(env string, obj *nais_io_v1alpha1.Application) ident.Ident {
		return newIdent(slug.Slug(obj.GetNamespace()), env, obj.GetName())
	}

	gbi := func(ctx context.Context, id ident.Ident) (search.SearchNode, error) {
		return GetByIdent(ctx, id)
	}

	search.RegisterBleve("APPLICATION", search.NewK8sSearch(watcher, gbi, createIdent))
}

// type searcher struct{}

// var _ search.Searchable = &searcher{}

// func (s searcher) Convert(ctx context.Context, ids ...ident.Ident) ([]search.SearchNode, error) {
// 	ret := make([]search.SearchNode, 0, len(ids))
// 	for _, id := range ids {
// 		sn, err := GetByIdent(ctx, id)
// 		if err != nil {
// 			return nil, err
// 		}
// 		ret = append(ret, sn)
// 	}

// 	return ret, nil
// }

// func (m searcher) ReIndex(ctx context.Context) []search.Document {
// 	apps := fromContext(ctx).appWatcher.All()
// 	docs := make([]search.Document, 0, len(apps))
// 	for _, app := range apps {
// 		team := slug.Slug(app.GetNamespace())

// 		docs = append(docs, search.Document{
// 			ID:   newIdent(team, app.Cluster, app.GetName()).String(),
// 			Name: app.GetName(),
// 			Team: team.String(),
// 		})
// 	}

// 	return docs
// }

// func (m searcher) Watch(ctx context.Context, indexer search.Indexer) error {
// 	watcher := fromContext(ctx).appWatcher

// 	watcher.OnUpdate(m.onUpdate(indexer))
// 	watcher.OnRemove(m.onRemove(indexer))
// 	return nil
// }

// func (m searcher) onUpdate(indexer search.Indexer) func(string, *nais_io_v1alpha1.Application) {
// 	return func(env string, app *nais_io_v1alpha1.Application) {
// 		team := slug.Slug(app.GetNamespace())
// 		indexer.Update(search.Document{
// 			ID:   newIdent(team, env, app.GetName()).String(),
// 			Name: app.GetName(),
// 			Team: team.String(),
// 		})
// 	}
// }

// func (m searcher) onRemove(indexer search.Indexer) func(string, *nais_io_v1alpha1.Application) {
// 	return func(env string, app *nais_io_v1alpha1.Application) {
// 		team := slug.Slug(app.GetNamespace())
// 		indexer.Remove(newIdent(team, env, app.GetName()))
// 	}
// }
