package webhook

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type webhookMetrics struct {
	deliveriesCounter   metric.Int64Counter
	durationHistogram   metric.Float64Histogram
	processedCounter    metric.Int64Counter
	autoDisabledCounter metric.Int64Counter
	queueSizeGauge      metric.Int64ObservableGauge
}

func newWebhookMetrics(q *webhooksql.Queries) (*webhookMetrics, error) {
	meter := otel.GetMeterProvider().Meter("webhook")

	deliveriesCounter, err := meter.Int64Counter(
		"nais_api_webhook_deliveries_total",
		metric.WithDescription("Total number of webhook deliveries attempted."),
	)
	if err != nil {
		return nil, fmt.Errorf("create deliveries counter: %w", err)
	}

	durationHistogram, err := meter.Float64Histogram(
		"nais_api_webhook_delivery_duration_seconds",
		metric.WithDescription("Webhook delivery latency in seconds."),
		metric.WithExplicitBucketBoundaries(0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("create duration histogram: %w", err)
	}

	processedCounter, err := meter.Int64Counter(
		"nais_api_webhook_events_processed_total",
		metric.WithDescription("Total number of outbox webhook deliveries processed by the dispatcher."),
	)
	if err != nil {
		return nil, fmt.Errorf("create processed counter: %w", err)
	}

	autoDisabledCounter, err := meter.Int64Counter(
		"nais_api_webhook_subscriptions_auto_disabled_total",
		metric.WithDescription("Total number of webhook subscriptions automatically disabled due to consecutive failures."),
	)
	if err != nil {
		return nil, fmt.Errorf("create auto-disabled counter: %w", err)
	}

	m := &webhookMetrics{
		deliveriesCounter:   deliveriesCounter,
		durationHistogram:   durationHistogram,
		processedCounter:    processedCounter,
		autoDisabledCounter: autoDisabledCounter,
	}

	queueSizeGauge, err := meter.Int64ObservableGauge(
		"nais_api_webhook_queue_size",
		metric.WithDescription("Current size of the webhook delivery queue grouped by status. Reported identically by every replica; aggregate with max()/avg(), not sum()."),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			rows, err := q.GetQueueSizeByStatus(ctx)
			if err != nil {
				return err
			}

			// Ensure every known status is reported, even if its count is currently 0.
			statuses := map[webhooksql.WebhookDeliveryStatus]int64{
				webhooksql.WebhookDeliveryStatusPending:   0,
				webhooksql.WebhookDeliveryStatusCompleted: 0,
				webhooksql.WebhookDeliveryStatusFailed:    0,
			}

			for _, row := range rows {
				statuses[row.Status] = row.Count
			}

			for status, count := range statuses {
				observer.Observe(count, metric.WithAttributes(
					attribute.String("status", string(status)),
				))
			}

			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create queue size gauge: %w", err)
	}

	m.queueSizeGauge = queueSizeGauge

	return m, nil
}
