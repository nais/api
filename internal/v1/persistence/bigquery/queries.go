package bigquery

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
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
	all, err := fromContext(ctx).k8sClient.getBigQueryDatasetsForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case BigQueryDatasetOrderFieldName:
			slices.SortStableFunc(all, func(a, b *BigQueryDataset) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case BigQueryDatasetOrderFieldEnvironment:
			slices.SortStableFunc(all, func(a, b *BigQueryDataset) int {
				return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
			})
		}
	}

	datasets := pagination.Slice(all, page)
	return pagination.NewConnection(datasets, page, int32(len(all))), nil
}
