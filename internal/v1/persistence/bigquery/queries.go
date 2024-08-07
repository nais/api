package bigquery

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*BigQueryDataset, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*BigQueryDataset, error) {
	return fromContext(ctx).datasetLoader.Load(ctx, resourceIdentifier{
		namespace:   teamSlug.String(),
		environment: environment,
		name:        name,
	})
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *BigQueryDatasetOrder) (*BigQueryDatasetConnection, error) {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())
	ret := watcher.Objects(all)

	if orderBy == nil {
		orderBy = &BigQueryDatasetOrder{
			Field:     BigQueryDatasetOrderFieldName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}
	switch orderBy.Field {
	case BigQueryDatasetOrderFieldName:
		slices.SortStableFunc(ret, func(a, b *BigQueryDataset) int {
			return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
		})
	case BigQueryDatasetOrderFieldEnvironment:
		slices.SortStableFunc(ret, func(a, b *BigQueryDataset) int {
			return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
		})
	}

	datasets := pagination.Slice(ret, page)
	return pagination.NewConnection(datasets, page, int32(len(ret))), nil
}
