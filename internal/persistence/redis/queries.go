package redis

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
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
	all := ListAllForTeam(ctx, teamSlug)
	orderRedisInstance(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*RedisInstance {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
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

	if orderBy == nil {
		orderBy = &RedisInstanceAccessOrder{Field: RedisInstanceAccessOrderFieldAccess, Direction: model.OrderDirectionAsc}
	}
	SortFilterRedisInstanceAccess.Sort(ctx, all, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(all, page)
	return pagination.NewConnection(ret, page, len(all)), nil
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

	orderRedisInstance(ctx, ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func Search(ctx context.Context, q string) ([]*search.Result, error) {
	apps := fromContext(ctx).client.watcher.All()

	ret := make([]*search.Result, 0)
	for _, app := range apps {
		rank := search.Match(q, app.Obj.Name)
		if search.Include(rank) {
			ret = append(ret, &search.Result{
				Rank: rank,
				Node: app.Obj,
			})
		}
	}

	return ret, nil
}

func orderRedisInstance(ctx context.Context, instances []*RedisInstance, orderBy *RedisInstanceOrder) {
	if orderBy == nil {
		orderBy = &RedisInstanceOrder{
			Field:     RedisInstanceOrderFieldName,
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilterRedisInstance.Sort(ctx, instances, orderBy.Field, orderBy.Direction)
}
