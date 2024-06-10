package bigquery

import (
	"go.opentelemetry.io/otel"
)

var (
	bigQueryDatasetListErrorCounter, _    = otel.Meter("").Int64Counter("bigQueryDataset_list_errors_total")
	bigQueryDatasetListOpsCounter, _      = otel.Meter("").Int64Counter("bigQueryDataset_list_ops_total")
	bigQueryDatasetErrorCounter, _        = otel.Meter("").Int64Counter("bigQueryDataset_errors_total")
	bigQueryDatasetOpsCounter, _          = otel.Meter("").Int64Counter("bigQueryDataset_ops_total")
	bigQueryDatasetCostOpsCounter, _      = otel.Meter("").Int64Counter("bigQueryDataset_cost_ops_total")
	bigQueryDatasetCostOpsErrorCounter, _ = otel.Meter("").Int64Counter("bigQueryDataset_cost_ops_error_total")
)
