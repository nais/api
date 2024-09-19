package redis

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/searchv1"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*RedisInstance, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*RedisInstance, error) {
	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *RedisInstanceOrder) (*RedisInstanceConnection, error) {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String())
	ret := watcher.Objects(all)

	orderRedisInstance(ret, orderBy)

	instances := pagination.Slice(ret, page)
	return pagination.NewConnection(instances, page, int32(len(ret))), nil
}

func ListAccess(ctx context.Context, redis *RedisInstance, page *pagination.Pagination, orderBy *RedisInstanceAccessOrder) (*RedisInstanceAccessConnection, error) {
	k8sClient := fromContext(ctx).client

	applicationAccess, err := k8sClient.getAccessForApplications(ctx, redis.EnvironmentName, redis.Name, redis.TeamSlug)
	if err != nil {
		return nil, err
	}

	jobAccess, err := k8sClient.getAccessForJobs(ctx, redis.EnvironmentName, redis.Name, redis.TeamSlug)
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
				return modelv1.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name, orderBy.Direction)
			})
		}
	}

	ret := pagination.Slice(all, page)
	return pagination.NewConnection(ret, page, int32(len(all))), nil
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, references []nais_io_v1.Redis, orderBy *RedisInstanceOrder) (*RedisInstanceConnection, error) {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String())
	ret := make([]*RedisInstance, 0)

	for _, ref := range references {
		for _, d := range all {
			if d.Obj.Name == redisInstanceNamer(teamSlug, ref.Instance) {
				ret = append(ret, d.Obj)
			}
		}
	}

	orderRedisInstance(ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func Search(ctx context.Context, q string) ([]*searchv1.Result, error) {
	apps := fromContext(ctx).client.watcher.All()

	ret := make([]*searchv1.Result, 0)
	for _, app := range apps {
		rank := searchv1.Match(q, app.Obj.Name)
		if searchv1.Include(rank) {
			ret = append(ret, &searchv1.Result{
				Rank: rank,
				Node: app.Obj,
			})
		}
	}

	return ret, nil
}

func orderRedisInstance(datasets []*RedisInstance, orderBy *RedisInstanceOrder) {
	if orderBy == nil {
		orderBy = &RedisInstanceOrder{
			Field:     RedisInstanceOrderFieldName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}
	switch orderBy.Field {
	case RedisInstanceOrderFieldName:
		slices.SortStableFunc(datasets, func(a, b *RedisInstance) int {
			return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
		})
	case RedisInstanceOrderFieldEnvironment:
		slices.SortStableFunc(datasets, func(a, b *RedisInstance) int {
			return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
		})
	}
}
