package opensearch

import (
	"go.opentelemetry.io/otel"
)

var (
	OpensearchListErrorCounter, _ = otel.Meter("").Int64Counter("opensearch_list_errors_total")
	OpensearchListOpsCounter, _   = otel.Meter("").Int64Counter("opensearch_list_ops_total")
	OpensearchErrorCounter, _     = otel.Meter("").Int64Counter("opensearch_errors_total")
	OpensearchOpsCounter, _       = otel.Meter("").Int64Counter("opensearch_ops_total")
	OpensearchCostOpsCounter, _   = otel.Meter("").Int64Counter("opensearch_cost_ops_total")
	OpensearchCostErrorCounter, _ = otel.Meter("").Int64Counter("opensearch_cost_error_total")
)
