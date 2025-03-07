package redis

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterRedisInstance       = sortfilter.New[*RedisInstance, RedisInstanceOrderField, struct{}]()
	SortFilterRedisInstanceAccess = sortfilter.New[*RedisInstanceAccess, RedisInstanceAccessOrderField, struct{}]()
)

func init() {
	SortFilterRedisInstance.RegisterSort("NAME", func(ctx context.Context, a, b *RedisInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilterRedisInstance.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *RedisInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")

	SortFilterRedisInstanceAccess.RegisterSort("ACCESS", func(ctx context.Context, a, b *RedisInstanceAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterRedisInstanceAccess.RegisterSort("WORKLOAD", func(ctx context.Context, a, b *RedisInstanceAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
