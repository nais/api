package tunnel

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("tunnel")

var tunnelOperationsTotal metric.Int64Counter

func init() {
	var err error
	tunnelOperationsTotal, err = meter.Int64Counter(
		"tunnel_api_operations_total",
		metric.WithDescription("Total number of tunnel operations."),
	)
	if err != nil {
		panic(err)
	}
}
