package application

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

type searcher struct{}

var _ search.Searchable = &searcher{}

func (s searcher) Convert(ctx context.Context, ids ...ident.Ident) ([]search.SearchNode, error) {
	ret := make([]search.SearchNode, 0, len(ids))
	for _, id := range ids {
		sn, err := GetByIdent(ctx, id)
		if err != nil {
			return nil, err
		}
		ret = append(ret, sn)
	}

	return ret, nil
}

func (m searcher) ReIndex(ctx context.Context) []search.Document {
	apps := fromContext(ctx).appWatcher.All()
	docs := make([]search.Document, 0, len(apps))
	for _, app := range apps {
		team := slug.Slug(app.GetNamespace())

		docs = append(docs, search.Document{
			ID:   newIdent(team, app.Cluster, app.GetName()).String(),
			Name: app.GetName(),
			Team: team.String(),
		})
	}

	return docs
}

func init() {
	search.Register("APPLICATION", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})

	search.RegisterBleve("APPLICATION", &searcher{})
}
