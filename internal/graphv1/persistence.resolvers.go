package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/redis"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
)

func (r *bigQueryDatasetResolver) Team(ctx context.Context, obj *bigquery.BigQueryDataset) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *bigQueryDatasetResolver) Environment(ctx context.Context, obj *bigquery.BigQueryDataset) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bigQueryDatasetResolver) Workload(ctx context.Context, obj *bigquery.BigQueryDataset) (workload.Workload, error) {
	panic(fmt.Errorf("not implemented: Workload - workload"))
}

func (r *bigQueryDatasetResolver) Cost(ctx context.Context, obj *bigquery.BigQueryDataset) (float64, error) {
	// Should we make cost a separate domain?
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *redisInstanceResolver) Access(ctx context.Context, obj *redis.RedisInstance) ([]*redis.RedisInstanceAccess, error) {
	panic(fmt.Errorf("not implemented: Access - access"))
}

func (r *redisInstanceResolver) Team(ctx context.Context, obj *redis.RedisInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *redisInstanceResolver) Environment(ctx context.Context, obj *redis.RedisInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *redisInstanceResolver) Cost(ctx context.Context, obj *redis.RedisInstance) (float64, error) {
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *Resolver) BigQueryDataset() gengqlv1.BigQueryDatasetResolver {
	return &bigQueryDatasetResolver{r}
}

func (r *Resolver) RedisInstance() gengqlv1.RedisInstanceResolver { return &redisInstanceResolver{r} }

type (
	bigQueryDatasetResolver struct{ *Resolver }
	redisInstanceResolver   struct{ *Resolver }
)