package bucket

import (
	"go.opentelemetry.io/otel"
)

var (
	bucketListErrorCounter, _    = otel.Meter("").Int64Counter("bucket_list_errors_total")
	bucketListOpsCounter, _      = otel.Meter("").Int64Counter("bucket_list_ops_total")
	bucketErrorCounter, _        = otel.Meter("").Int64Counter("bucket_errors_total")
	bucketOpsCounter, _          = otel.Meter("").Int64Counter("bucket_ops_total")
	bucketCostOpsCounter, _      = otel.Meter("").Int64Counter("bigQueryDataset_cost_ops_total")
	bucketCostOpsErrorCounter, _ = otel.Meter("").Int64Counter("bigQueryDataset_cost_ops_error_total")
)
