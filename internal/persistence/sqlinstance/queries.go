package sqlinstance

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
	"google.golang.org/api/googleapi"
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

func ListSQLInstanceUsers(ctx context.Context, sqlInstance *SQLInstance, page *pagination.Pagination, orderBy *SQLInstanceUserOrder) (*SQLInstanceUserConnection, error) {
	adminUsers, err := fromContext(ctx).sqlAdminService.GetUsers(ctx, sqlInstance.ProjectID, sqlInstance.Name)
	if err != nil {
		var googleErr *googleapi.Error
		if errors.As(err, &googleErr) && googleErr.Code == 400 {
			// TODO: This was handled in the legacy code, keep it for now. Log?
			return nil, nil
		}
		return nil, fmt.Errorf("getting SQL users")
	}

	all := make([]*SQLInstanceUser, len(adminUsers))
	for i, user := range adminUsers {
		all[i] = toSQLInstanceUser(user)
	}

	if orderBy != nil {
		switch orderBy.Field {
		case SQLInstanceUserOrderFieldName:
			slices.SortStableFunc(all, func(a, b *SQLInstanceUser) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case SQLInstanceUserOrderFieldAuthentication:
			slices.SortStableFunc(all, func(a, b *SQLInstanceUser) int {
				return modelv1.Compare(a.Authentication, b.Authentication, orderBy.Direction)
			})
		}
	}

	users := pagination.Slice(all, page)
	return pagination.NewConnection(users, page, int32(len(all))), nil
}
