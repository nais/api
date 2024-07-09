package redis

import (
	"context"
	"slices"

	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*RedisInstance, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*RedisInstance, error) {
	return fromContext(ctx).redisLoader.Load(ctx, resourceIdentifier{
		namespace:   teamSlug.String(),
		environment: environment,
		name:        name,
	})
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *RedisInstanceOrder) (*RedisInstanceConnection, error) {
	all, err := fromContext(ctx).k8sClient.getRedisInstancesForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case RedisInstanceOrderFieldName:
			slices.SortStableFunc(all, func(a, b *RedisInstance) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case RedisInstanceOrderFieldEnvironment:
			slices.SortStableFunc(all, func(a, b *RedisInstance) int {
				return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
			})
		}
	}

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, int32(len(all))), nil
}

func ListAccess(ctx context.Context, redis *RedisInstance, page *pagination.Pagination, orderBy *RedisInstanceAccessOrder) (*RedisInstanceAccessConnection, error) {
	k8sClient := fromContext(ctx).k8sClient

	applicationAccess, err := k8sClient.getAccessForApplications(redis.EnvironmentName, redis.Name, redis.TeamSlug)
	if err != nil {
		return nil, err
	}

	jobAccess, err := k8sClient.getAccessForJobs(redis.EnvironmentName, redis.Name, redis.TeamSlug)
	if err != nil {
		return nil, err
	}

	all := make([]*RedisInstanceAccess, 0)
	all = append(all, applicationAccess...)
	all = append(all, jobAccess...)

	if orderBy != nil {
		switch orderBy.Field {
		case RedisInstanceAccessOrderFieldAccess:
			slices.SortStableFunc(all, func(a, b *RedisInstanceAccess) int {
				return modelv1.Compare(a.Access, b.Access, orderBy.Direction)
			})
		case RedisInstanceAccessOrderFieldWorkload:
			slices.SortStableFunc(all, func(a, b *RedisInstanceAccess) int {
				return modelv1.Compare(a.OwnerReference.Name, b.OwnerReference.Name, orderBy.Direction)
			})
		}
	}

	ret := pagination.Slice(all, page)
	return pagination.NewConnection(ret, page, int32(len(all))), nil
}
