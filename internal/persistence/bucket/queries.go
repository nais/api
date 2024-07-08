package bucket

import (
	"context"
	"slices"

	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*Bucket, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Bucket, error) {
	return fromContext(ctx).datasetLoader.Load(ctx, resourceIdentifier{
		namespace:   teamSlug.String(),
		environment: environment,
		name:        name,
	})
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *BucketOrder) (*BucketConnection, error) {
	all, err := fromContext(ctx).k8sClient.getBucketsForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case BucketOrderFieldName:
			slices.SortStableFunc(all, func(a, b *Bucket) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case BucketOrderFieldEnvironment:
			slices.SortStableFunc(all, func(a, b *Bucket) int {
				return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
			})
		}
	}

	slice := pagination.Slice(all, page)
	return pagination.NewConnection(slice, page, int32(len(all))), nil
}
