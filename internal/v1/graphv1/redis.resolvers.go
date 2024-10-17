package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/persistence/redis"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *applicationResolver) RedisInstances(ctx context.Context, obj *application.Application, orderBy *redis.RedisInstanceOrder) (*pagination.Connection[*redis.RedisInstance], error) {
	return redis.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.Redis, orderBy)
}

func (r *jobResolver) RedisInstances(ctx context.Context, obj *job.Job, orderBy *redis.RedisInstanceOrder) (*pagination.Connection[*redis.RedisInstance], error) {
	return redis.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.Redis, orderBy)
}

func (r *redisInstanceResolver) Team(ctx context.Context, obj *redis.RedisInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *redisInstanceResolver) Environment(ctx context.Context, obj *redis.RedisInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *redisInstanceResolver) Access(ctx context.Context, obj *redis.RedisInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *redis.RedisInstanceAccessOrder) (*pagination.Connection[*redis.RedisInstanceAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return redis.ListAccess(ctx, obj, page, orderBy)
}

func (r *redisInstanceResolver) Workload(ctx context.Context, obj *redis.RedisInstance) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *redisInstanceAccessResolver) Workload(ctx context.Context, obj *redis.RedisInstanceAccess) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *teamResolver) RedisInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *redis.RedisInstanceOrder) (*pagination.Connection[*redis.RedisInstance], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return redis.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) RedisInstance(ctx context.Context, obj *team.TeamEnvironment, name string) (*redis.RedisInstance, error) {
	return redis.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamInventoryCountsResolver) RedisInstances(ctx context.Context, obj *team.TeamInventoryCounts) (*redis.TeamInventoryCountRedisInstances, error) {
	return &redis.TeamInventoryCountRedisInstances{
		Total: len(redis.ListAllForTeam(ctx, obj.TeamSlug)),
	}, nil
}

func (r *Resolver) RedisInstance() gengqlv1.RedisInstanceResolver { return &redisInstanceResolver{r} }

func (r *Resolver) RedisInstanceAccess() gengqlv1.RedisInstanceAccessResolver {
	return &redisInstanceAccessResolver{r}
}

type (
	redisInstanceResolver       struct{ *Resolver }
	redisInstanceAccessResolver struct{ *Resolver }
)
