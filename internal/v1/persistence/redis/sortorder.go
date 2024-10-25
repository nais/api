package redis

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/sortfilter"
)

var (
	SortFilterRedisInstance       = sortfilter.New[*RedisInstance, RedisInstanceOrderField, struct{}](RedisInstanceOrderFieldName)
	SortFilterRedisInstanceAccess = sortfilter.New[*RedisInstanceAccess, RedisInstanceAccessOrderField, struct{}](RedisInstanceAccessOrderFieldAccess)
)

func init() {
	SortFilterRedisInstance.RegisterOrderBy(RedisInstanceOrderFieldName, func(ctx context.Context, a, b *RedisInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterRedisInstance.RegisterOrderBy(RedisInstanceOrderFieldEnvironment, func(ctx context.Context, a, b *RedisInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterRedisInstanceAccess.RegisterOrderBy(RedisInstanceAccessOrderFieldAccess, func(ctx context.Context, a, b *RedisInstanceAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterRedisInstanceAccess.RegisterOrderBy(RedisInstanceAccessOrderFieldWorkload, func(ctx context.Context, a, b *RedisInstanceAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
