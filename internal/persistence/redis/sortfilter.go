package redis

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterRedisInstance       = sortfilter.New[*RedisInstance, RedisInstanceOrderField, struct{}]("NAME", model.OrderDirectionAsc)
	SortFilterRedisInstanceAccess = sortfilter.New[*RedisInstanceAccess, RedisInstanceAccessOrderField, struct{}]("ACCESS", model.OrderDirectionAsc)
)

func init() {
	SortFilterRedisInstance.RegisterOrderBy("NAME", func(ctx context.Context, a, b *RedisInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterRedisInstance.RegisterOrderBy("ENVIRONMENT", func(ctx context.Context, a, b *RedisInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterRedisInstanceAccess.RegisterOrderBy("ACCESS", func(ctx context.Context, a, b *RedisInstanceAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterRedisInstanceAccess.RegisterOrderBy("WORKLOAD", func(ctx context.Context, a, b *RedisInstanceAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
