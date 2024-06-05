package kafka

import (
	"go.opentelemetry.io/otel"
)

var (
	KafkaListErrorCounter, _ = otel.Meter("").Int64Counter("kafka_list_errors_total")
	KafkaListOpsCounter, _   = otel.Meter("").Int64Counter("kafka_list_ops_total")
	KafkaErrorCounter, _     = otel.Meter("").Int64Counter("kafka_errors_total")
	KafkaOpsCounter, _       = otel.Meter("").Int64Counter("kafka_ops_total")
)
