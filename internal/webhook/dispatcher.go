package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/database/notify"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/webhook/webhooksql"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	defaultTimeout    = 10 * time.Second
	eventBatchSize    = 50
	pollInterval      = 30 * time.Second
	maxRetryCount     = 7  // ~24h total with exponential backoff
	disableThreshold  = 10 // auto-disable after 10 consecutive failures
	userAgentHeader   = "Nais-API-Webhook/1.0"
	signatureHeader   = "X-Webhook-Signature"
	contentTypeHeader = "application/cloudevents+json"
)

// retryBackoffs defines how long to wait before retrying at each retry_count.
// Total span: ~24 hours.
var retryBackoffs = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	4 * time.Hour,
	8 * time.Hour,
	12 * time.Hour,
}

// Dispatcher processes webhook events from the outbox table and delivers them to subscribers.
type Dispatcher struct {
	pool       *pgxpool.Pool
	notifier   *notify.Notifier
	log        logrus.FieldLogger
	source     string
	httpClient *http.Client
	metrics    *webhookMetrics
}

// NewDispatcher creates a new webhook dispatcher that drains events from the outbox table.
func NewDispatcher(pool *pgxpool.Pool, notifier *notify.Notifier, source string, log logrus.FieldLogger) (*Dispatcher, error) {
	q := webhooksql.New(pool)
	m, err := newWebhookMetrics(q)
	if err != nil {
		return nil, fmt.Errorf("setting up webhook metrics: %w", err)
	}

	return &Dispatcher{
		pool:     pool,
		notifier: notifier,
		log:      log.WithField("subsystem", "webhook_dispatcher"),
		source:   source,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		metrics: m,
	}, nil
}

// Run starts the dispatcher. It listens for PG NOTIFY on "webhook_events" and
// periodically polls for unprocessed events. Blocks until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) {
	ch := d.notifier.Listen("webhook_events")

	// Process any events that were queued before we started
	d.drainOutbox(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			d.drainOutbox(ctx)
		case <-time.After(pollInterval):
			// Safety net: poll periodically in case a notification was missed
			// or to pick up events whose run_at has arrived
			d.drainOutbox(ctx)
		}
	}
}

func (d *Dispatcher) drainOutbox(ctx context.Context) {
	q := webhooksql.New(d.pool)

	for {
		events, err := q.ClaimPendingEvents(ctx, eventBatchSize)
		if err != nil {
			d.log.WithError(err).Error("claiming pending webhook events")
			return
		}

		if len(events) == 0 {
			return
		}

		for _, evt := range events {
			d.processEvent(ctx, q, &evt.WebhookEvent, &evt.ActivityLogEntry)
		}
	}
}

func (d *Dispatcher) processEvent(ctx context.Context, q *webhooksql.Queries, evt *webhooksql.WebhookEvent, a *webhooksql.ActivityLogEntry) {
	subs, err := q.ListEnabledSubscriptions(ctx)
	if err != nil {
		d.log.WithError(err).Error("listing enabled webhook subscriptions")
		return
	}

	// Resolve "RESOURCE_TYPE:ACTION" → ActivityLogActivityType names
	// (e.g. ResourceType="TEAM", Action="ADDED" → ["TEAM_MEMBER_ADDED"]).
	rawEventType := a.ResourceType + ":" + a.Action
	resolved := activitylog.LookupActivityTypes(a.ResourceType, a.Action)
	activityTypes := make([]string, len(resolved))
	for i, at := range resolved {
		activityTypes[i] = string(at)
	}
	// Fall back to raw type if no mapping is registered, so the event is still deliverable.
	if len(activityTypes) == 0 {
		activityTypes = []string{rawEventType}
	}

	var teamSlug *slug.Slug
	if a.TeamSlug != nil {
		s := slug.Slug(*a.TeamSlug)
		teamSlug = &s
	}

	event := WebhookEvent{
		ActivityTypes: activityTypes,
		RawEventType:  rawEventType,
		TeamSlug:      teamSlug,
		Actor:         a.Actor,
		ResourceType:  a.ResourceType,
		ResourceName:  a.ResourceName,
		Environment:   a.Environment,
		Data:          a.Data,
	}

	payload, err := BuildCloudEvent(d.source, event)
	if err != nil {
		d.log.WithError(err).Error("building CloudEvent payload")
		return
	}

	// Use the first resolved activity type as the delivery event type label.
	deliveryEventType := activityTypes[0]

	anyFailed := false
	for _, sub := range subs {
		graphSub := toGraphSubscription(sub)
		if !graphSub.MatchesEvent(event) {
			continue
		}

		success := d.deliver(ctx, q, sub, deliveryEventType, payload)
		if !success {
			anyFailed = true
		}
	}

	// If any delivery failed, requeue with exponential backoff or mark as permanently failed.
	if anyFailed {
		nextRetry := int(evt.RetryCount) + 1
		if nextRetry <= maxRetryCount {
			backoff := retryBackoffs[min(nextRetry-1, len(retryBackoffs)-1)]
			runAt := time.Now().Add(backoff)
			if err := q.RequeueEvent(ctx, webhooksql.RequeueEventParams{
				ID:         evt.ID,
				RetryCount: int32(nextRetry),
				RunAt:      pgtype.Timestamptz{Time: runAt, Valid: true},
			}); err != nil {
				d.log.WithError(err).Error("requeueing webhook event")
			}
			d.metrics.processedCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "requeued"),
			))
		} else {
			if err := q.MarkEventFailed(ctx, evt.ID); err != nil {
				d.log.WithError(err).Error("marking webhook event as failed")
			}
			d.metrics.processedCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "failed"),
			))
		}
	} else {
		d.metrics.processedCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "completed"),
		))
	}
}

func (d *Dispatcher) deliver(ctx context.Context, q *webhooksql.Queries, sub *webhooksql.WebhookSubscription, eventType string, payload []byte) bool {
	signature := SignPayload(sub.Secret, payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Url, bytes.NewReader(payload))
	if err != nil {
		d.log.WithError(err).WithField("subscription_id", sub.ID).Error("creating HTTP request")
		return false
	}

	req.Header.Set("Content-Type", contentTypeHeader)
	req.Header.Set("User-Agent", userAgentHeader)
	req.Header.Set(signatureHeader, signature)

	start := time.Now()
	resp, err := d.httpClient.Do(req)
	durationSeconds := time.Since(start).Seconds()
	durationMs := int32(durationSeconds * 1000)

	var (
		responseStatus *int32
		responseBody   *string
		success        bool
	)

	statusStr := "network_error"
	if err != nil {
		errMsg := err.Error()
		responseBody = &errMsg
	} else {
		defer resp.Body.Close()
		status := int32(resp.StatusCode)
		responseStatus = &status
		statusStr = strconv.Itoa(int(resp.StatusCode))
		success = resp.StatusCode >= 200 && resp.StatusCode < 300

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*10)) // 10KB max
		if readErr == nil {
			bodyStr := string(body)
			responseBody = &bodyStr
		}
	}

	successStr := "false"
	if success {
		successStr = "true"
	}

	d.metrics.deliveriesCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("event_type", eventType),
		attribute.String("status_code", statusStr),
		attribute.String("success", successStr),
	))

	d.metrics.durationHistogram.Record(ctx, durationSeconds, metric.WithAttributes(
		attribute.String("event_type", eventType),
	))

	// Record delivery attempt
	if _, recordErr := q.CreateDelivery(ctx, webhooksql.CreateDeliveryParams{
		SubscriptionID: sub.ID,
		EventType:      eventType,
		RequestBody:    payload,
		ResponseStatus: responseStatus,
		ResponseBody:   responseBody,
		DurationMs:     durationMs,
		Success:        success,
	}); recordErr != nil {
		d.log.WithError(recordErr).Error("recording webhook delivery")
	}

	// Track consecutive failures for auto-disable
	if success {
		if sub.ConsecutiveFailures > 0 {
			if err := q.ResetConsecutiveFailures(ctx, sub.ID); err != nil {
				d.log.WithError(err).Error("resetting consecutive failures")
			}
		}
	} else {
		updated, err := q.IncrementConsecutiveFailures(ctx, sub.ID)
		if err != nil {
			d.log.WithError(err).Error("incrementing consecutive failures")
		} else if updated.ConsecutiveFailures >= disableThreshold {
			d.log.WithField("subscription_id", sub.ID).Warn("auto-disabling webhook subscription after repeated failures")
			if err := q.DisableSubscription(ctx, sub.ID); err != nil {
				d.log.WithError(err).Error("disabling webhook subscription")
			}
			d.metrics.autoDisabledCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("subscription_id", sub.ID.String()),
			))
		}
	}

	return success
}

// SerializeEventData is a helper to serialize event data to JSON for the webhook payload.
func SerializeEventData(data any) ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	return json.Marshal(data)
}

// Ping sends a test ping payload to the given subscription and records the delivery.
// Used to verify connectivity when a new webhook is registered.
func (d *Dispatcher) Ping(ctx context.Context, sub *WebhookSubscription) error {
	pingEvent := WebhookEvent{
		RawEventType: "ping",
		TeamSlug:     sub.TeamSlug,
		Actor:        "system",
		ResourceType: "webhook",
		ResourceName: sub.UUID.String(),
	}

	payload, err := BuildCloudEvent(d.source, pingEvent)
	if err != nil {
		return fmt.Errorf("building ping CloudEvent: %w", err)
	}

	q := webhooksql.New(d.pool)
	dbSub := &webhooksql.WebhookSubscription{
		ID:     sub.UUID,
		Url:    sub.URL,
		Secret: sub.Secret,
	}
	d.deliver(ctx, q, dbSub, "ping", payload)
	return nil
}
