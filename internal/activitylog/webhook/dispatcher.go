package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	"github.com/nais/api/internal/database/notify"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	defaultTimeout    = 10 * time.Second
	eventBatchSize    = 50
	deliveryWorkers   = 3
	pollInterval      = 30 * time.Second
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
var maxRetryCount = len(retryBackoffs)

// Dispatcher processes webhook events from the outbox table and delivers them to subscribers.
type Dispatcher struct {
	pool       *pgxpool.Pool
	queries    webhooksql.Querier
	notifier   *notify.Notifier
	log        logrus.FieldLogger
	source     string
	httpClient *http.Client
	metrics    *webhookMetrics
	wakeups    [deliveryWorkers]chan struct{}
}

// NewDispatcher creates a new webhook dispatcher that drains events from the outbox table.
func NewDispatcher(pool *pgxpool.Pool, notifier *notify.Notifier, source string, log logrus.FieldLogger) (*Dispatcher, error) {
	q := webhooksql.New(pool)
	m, err := newWebhookMetrics(q)
	if err != nil {
		return nil, fmt.Errorf("setting up webhook metrics: %w", err)
	}

	d := &Dispatcher{
		pool:     pool,
		queries:  q,
		notifier: notifier,
		log:      log.WithField("subsystem", "webhook_dispatcher"),
		source:   source,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		metrics: m,
	}
	for i := range d.wakeups {
		d.wakeups[i] = make(chan struct{}, 1)
	}
	return d, nil
}

// Run starts the dispatcher. It listens for PG NOTIFY on "webhook_events" and
// periodically polls for unprocessed events. It returns after cancellation and active deliveries finish.
func (d *Dispatcher) Run(ctx context.Context) {
	ch := d.notifier.Listen("webhook_events")
	workers := d.startDeliveryWorkers(ctx)
	defer workers.Wait()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	d.fanOutPendingEvents(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			d.fanOutPendingEvents(ctx)
		case <-ticker.C:
			d.fanOutPendingEvents(ctx)
		}
	}
}

func (d *Dispatcher) startDeliveryWorkers(ctx context.Context) *sync.WaitGroup {
	var workers sync.WaitGroup
	for _, wakeup := range d.wakeups {
		workers.Go(func() {
			d.runDeliveryWorker(ctx, wakeup)
		})
	}
	return &workers
}

func (d *Dispatcher) wakeDeliveryWorkers() {
	for _, wakeup := range d.wakeups {
		select {
		case wakeup <- struct{}{}:
		default:
		}
	}
}

func (d *Dispatcher) runDeliveryWorker(ctx context.Context, wakeup <-chan struct{}) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		d.drainPendingDeliveries(ctx)
		select {
		case <-ctx.Done():
			return
		case <-wakeup:
		case <-ticker.C:
		}
	}
}

// fanOutPendingEvents claims batches of outbox events that haven't been matched against
// subscriptions yet, and creates one webhook_event_deliveries row per currently enabled
// subscription that matches. Subscription matching only happens here; every later retry
// operates on a single (event, subscription) delivery row.
func (d *Dispatcher) fanOutPendingEvents(ctx context.Context) {
	for ctx.Err() == nil {
		more, err := d.fanOutBatch(ctx)
		if err != nil {
			d.log.WithError(err).Error("fanning out webhook outbox events")
			return
		}
		if !more {
			return
		}
		d.wakeDeliveryWorkers()
	}
}

func (d *Dispatcher) fanOutBatch(ctx context.Context) (bool, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("beginning fan-out transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op once committed

	q := webhooksql.New(d.pool).WithTx(tx)

	claimed, err := q.ClaimOutboxEventsForFanout(ctx, eventBatchSize)
	if err != nil {
		return false, fmt.Errorf("claiming outbox events: %w", err)
	}
	if len(claimed) == 0 {
		return false, nil
	}

	subs, err := q.ListEnabledSubscriptions(ctx)
	if err != nil {
		return false, fmt.Errorf("listing enabled webhook subscriptions: %w", err)
	}

	ids := make([]uuid.UUID, 0, len(claimed))
	for _, row := range claimed {
		ids = append(ids, row.WebhookEvent.ID)

		event := toWebhookEvent(&row.ActivityLogEntry)
		for _, sub := range subs {
			if !toGraphSubscription(sub).MatchesEvent(event) {
				continue
			}

			if err := q.CreateEventDelivery(ctx, webhooksql.CreateEventDeliveryParams{
				WebhookEventID: row.WebhookEvent.ID,
				SubscriptionID: sub.ID,
			}); err != nil {
				return false, fmt.Errorf("creating event delivery: %w", err)
			}
		}
	}

	// Fan-out and marking the event completed happen in the same transaction, so a failed
	// or interrupted attempt simply leaves the event pending for a later retry.
	if err := q.MarkOutboxEventsCompleted(ctx, ids); err != nil {
		return false, fmt.Errorf("marking outbox events fanned out: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("committing fan-out transaction: %w", err)
	}

	return true, nil
}

// drainPendingDeliveries claims one delivery at a time so each worker only owns its active job.
func (d *Dispatcher) drainPendingDeliveries(ctx context.Context) {
	for ctx.Err() == nil {
		deliveries, err := d.queries.ClaimPendingDeliveries(ctx, 1)
		if err != nil {
			d.log.WithError(err).Error("claiming pending webhook deliveries")
			return
		}

		if len(deliveries) == 0 {
			return
		}

		for _, row := range deliveries {
			// Finish claimed work on shutdown, including its database bookkeeping.
			deliveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*defaultTimeout)
			d.processDelivery(deliveryCtx, d.queries, &row.WebhookEventDelivery, &row.WebhookSubscription, &row.ActivityLogEntry)
			cancel()
		}
	}
}

// toWebhookEvent builds the internal WebhookEvent representation (used both for matching
// subscriptions and for building the CloudEvent payload) from a stored activity log entry.
func toWebhookEvent(a *webhooksql.ActivityLogEntry) WebhookEvent {
	rawEventType := a.ResourceType + ":" + a.Action

	// Resolve "RESOURCE_TYPE:ACTION" → ActivityLogActivityType names
	// (e.g. ResourceType="TEAM", Action="ADDED" → ["TEAM_MEMBER_ADDED"]).
	resolved := activitylog.LookupActivityTypes(a.ResourceType, a.Action)
	activityTypes := make([]string, len(resolved))
	for i, at := range resolved {
		activityTypes[i] = string(at)
	}
	// Fall back to the raw type if no mapping is registered, so the event is still deliverable.
	if len(activityTypes) == 0 {
		activityTypes = []string{rawEventType}
	}

	var teamSlug *slug.Slug
	if a.TeamSlug != nil {
		s := slug.Slug(*a.TeamSlug)
		teamSlug = &s
	}

	return WebhookEvent{
		ActivityTypes: activityTypes,
		RawEventType:  rawEventType,
		TeamSlug:      teamSlug,
		Actor:         a.Actor,
		ResourceType:  a.ResourceType,
		ResourceName:  a.ResourceName,
		Environment:   a.Environment,
		Data:          a.Data,
	}
}

func (d *Dispatcher) processDelivery(ctx context.Context, q webhooksql.Querier, del *webhooksql.WebhookEventDelivery, sub *webhooksql.WebhookSubscription, a *webhooksql.ActivityLogEntry) {
	event := toWebhookEvent(a)
	if event.isIgnored() {
		return
	}

	// Use the first resolved activity type as the delivery event type label.
	eventType := event.RawEventType
	if len(event.ActivityTypes) > 0 {
		eventType = event.ActivityTypes[0]
	}

	// The CloudEvent id is derived from the delivery row's own id, so it stays stable across retries.
	payload, err := BuildCloudEvent(d.source, del.ID.String(), event)
	if err != nil {
		d.log.WithError(err).Error("building CloudEvent payload")
		return
	}

	success := d.deliver(ctx, q, sub, eventType, payload, &del.ID)

	if success {
		d.metrics.processedCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "completed"),
		))
		return
	}

	// Requeue this delivery with exponential backoff, or mark it permanently failed once
	// the retry budget is exhausted.
	nextRetry := int(del.RetryCount) + 1
	if nextRetry <= maxRetryCount {
		backoff := retryBackoffs[min(nextRetry-1, len(retryBackoffs)-1)]
		runAt := time.Now().Add(backoff)
		if err := q.RequeueDelivery(ctx, webhooksql.RequeueDeliveryParams{
			ID:         del.ID,
			RetryCount: int32(nextRetry),
			RunAt:      pgtype.Timestamptz{Time: runAt, Valid: true},
		}); err != nil {
			d.log.WithError(err).Error("requeueing webhook delivery")
		}
		d.metrics.processedCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "requeued"),
		))
	} else {
		if err := q.MarkDeliveryFailed(ctx, del.ID); err != nil {
			d.log.WithError(err).Error("marking webhook delivery as failed")
		}
		d.metrics.processedCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "failed"),
		))
	}
}

// deliver sends a single HTTP delivery attempt to sub and records it in the audit log.
// deliveryRowID links the audit row back to the originating webhook_event_deliveries row,
// and is nil for ad hoc deliveries (e.g. Ping) that aren't backed by a queue row.
func (d *Dispatcher) deliver(ctx context.Context, q webhooksql.Querier, sub *webhooksql.WebhookSubscription, eventType string, payload []byte, deliveryRowID *uuid.UUID) bool {
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
		SubscriptionID:         sub.ID,
		WebhookEventDeliveryID: deliveryRowID,
		EventType:              eventType,
		RequestBody:            payload,
		ResponseStatus:         responseStatus,
		ResponseBody:           responseBody,
		DurationMs:             durationMs,
		Success:                success,
	}); recordErr != nil {
		d.log.WithError(recordErr).Error("recording webhook delivery")
	}

	autoDisabled, err := q.RecordDeliveryResult(ctx, webhooksql.RecordDeliveryResultParams{
		ID:               sub.ID,
		Success:          success,
		DisableThreshold: disableThreshold,
	})
	if err != nil {
		d.log.WithError(err).Error("recording webhook subscription delivery result")
	} else if autoDisabled {
		d.log.WithField("subscription_id", sub.ID).Warn("auto-disabling webhook subscription after repeated failures")
		d.metrics.autoDisabledCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("subscription_id", sub.ID.String()),
		))
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

	payload, err := BuildCloudEvent(d.source, uuid.New().String(), pingEvent)
	if err != nil {
		return fmt.Errorf("building ping CloudEvent: %w", err)
	}

	q := webhooksql.New(d.pool)
	dbSub := &webhooksql.WebhookSubscription{
		ID:     sub.UUID,
		Url:    sub.URL,
		Secret: sub.Secret,
	}
	d.deliver(ctx, q, dbSub, "ping", payload, nil)
	return nil
}
