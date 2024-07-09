package sqlinstance

import (
	"context"
	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*SQLInstance, error) {
	teamSlug, environmentName, sqlInstanceName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environmentName, sqlInstanceName)
}

func Get(ctx context.Context, teamSlug slug.Slug, environmentName, sqlInstanceName string) (*SQLInstance, error) {
	return fromContext(ctx).instanceLoader.Load(ctx, identifier{
		namespace:       teamSlug.String(),
		environmentName: environmentName,
		sqlInstanceName: sqlInstanceName,
	})
}

func GetDatabaseByIdent(ctx context.Context, id ident.Ident) (*SQLDatabase, error) {
	teamSlug, environmentName, sqlInstanceName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return GetDatabase(ctx, teamSlug, environmentName, sqlInstanceName)
}

func GetDatabase(ctx context.Context, teamSlug slug.Slug, environmentName, sqlInstanceName string) (*SQLDatabase, error) {
	return fromContext(ctx).databaseLoader.Load(ctx, identifier{
		namespace:       teamSlug.String(),
		environmentName: environmentName,
		sqlInstanceName: sqlInstanceName,
	})
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*SQLInstanceConnection, error) {
	all, err := fromContext(ctx).k8sClient.getInstancesForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, int32(len(all))), nil
}
