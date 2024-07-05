package bigquery

import (
	"context"

	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
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

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*BigQueryDatasetConnection, error) {
	all, err := fromContext(ctx).k8sClient.getBigQueryDatasetsForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	apps := pagination.Slice(all, page)
	return pagination.NewConnection(apps, page, int32(len(all))), nil
}
