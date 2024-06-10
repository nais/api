package sqlinstance

import (
	"go.opentelemetry.io/otel"
)

var (
	SqlInstanceListErrorCounter, _ = otel.Meter("").Int64Counter("sqlInstance_list_errors_total")
	SqlInstanceListOpsCounter, _   = otel.Meter("").Int64Counter("sqlInstance_list_ops_total")
	SqlInstanceErrorCounter, _     = otel.Meter("").Int64Counter("sqlInstance_errors_total")
	SqlInstanceOpsCounter, _       = otel.Meter("").Int64Counter("sqlInstance_ops_total")
)
