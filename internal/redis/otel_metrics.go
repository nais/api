package redis

import (
	"go.opentelemetry.io/otel"
)

var (
	RedisListErrorCounter, _ = otel.Meter("").Int64Counter("redis_list_errors_total")
	RedisListOpsCounter, _   = otel.Meter("").Int64Counter("redis_list_ops_total")
	RedisErrorCounter, _     = otel.Meter("").Int64Counter("redis_errors_total")
	RedisOpsCounter, _       = otel.Meter("").Int64Counter("redis_ops_total")
	RedisCostOpsCounter, _   = otel.Meter("").Int64Counter("redis_cost_ops_total")
	RedisCostErrorCounter, _ = otel.Meter("").Int64Counter("redis_cost_error_total")
)
