package opensearch

import (
	"context"
	"slices"

	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*OpenSearch, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*OpenSearch, error) {
	return fromContext(ctx).openSearchLoader.Load(ctx, resourceIdentifier{
		namespace:   teamSlug.String(),
		environment: environment,
		name:        name,
	})
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *OpenSearchOrder) (*OpenSearchConnection, error) {
	all, err := fromContext(ctx).k8sClient.getOpenSearchesForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case OpenSearchOrderFieldName:
			slices.SortStableFunc(all, func(a, b *OpenSearch) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case OpenSearchOrderFieldEnvironment:
			slices.SortStableFunc(all, func(a, b *OpenSearch) int {
				return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
			})
		}
	}

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, int32(len(all))), nil
}
