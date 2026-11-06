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
	defaultTimeout      = 10 * time.Second
	eventBatchSize      = 50
	deliveryWorkers     = 3
	pollInterval        = 30 * time.Second
	disableThreshold    = 10 // auto-disable after 10 consecutive failures
	userAgentHeader     = "Nais-API-Webhook/1.0"
	signatureHeader     = "X-Webhook-Signature"
	contentTypeHeader   = "application/cloudevents+json"
	maxResponseBodySize = 10 * 1024
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
func NewDispatcher(pool *pgxpool.Pool, notifier *notify.Notifier, source, tenant string, log logrus.FieldLogger) (*Dispatcher, error) {
	q := webhooksql.New(pool)
	m, err := newWebhookMetrics(q)
	if err != nil {
		return nil, fmt.Errorf("setting up webhook metrics: %w", err)
	}

	d := &Dispatcher{
		pool:       pool,
		queries:    q,
		notifier:   notifier,
		log:        log.WithField("subsystem", "webhook_dispatcher"),
		source:     source,
		httpClient: newWebhookHTTPClient(tenant),
		metrics:    m,
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
		ActivityLogEntryID: &a.ID,
		TriggeredAt:        a.CreatedAt.Time,
		ActivityTypes:      activityTypes,
		RawEventType:       rawEventType,
		TeamSlug:           teamSlug,
		Actor:              a.Actor,
		ResourceType:       a.ResourceType,
		ResourceName:       a.ResourceName,
		Environment:        a.Environment,
		Data:               a.Data,
	}
}

func (d *Dispatcher) processDelivery(ctx context.Context, q webhooksql.Querier, del *webhooksql.WebhookEventDelivery, sub *webhooksql.WebhookSubscription, a *webhooksql.ActivityLogEntry) {
	event := toWebhookEvent(a)
	if event.isIgnored() {
		if err := q.MarkDeliveryCompleted(ctx, webhooksql.MarkDeliveryCompletedParams{ID: del.ID, ClaimToken: del.ClaimToken}); err != nil {
			d.log.WithError(err).Error("finishing ignored webhook delivery")
		}
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
		if err := q.MarkDeliveryFailed(ctx, webhooksql.MarkDeliveryFailedParams{ID: del.ID, ClaimToken: del.ClaimToken}); err != nil {
			d.log.WithError(err).Error("marking invalid webhook delivery failed")
		}
		return
	}

	attempt, err := d.deliver(ctx, q, sub, eventType, payload, &del.ID)
	if err != nil {
		d.log.WithError(err).Error("recording webhook delivery attempt")
	}
	success := attempt != nil && attempt.Success

	if success {
		if err := q.MarkDeliveryCompleted(ctx, webhooksql.MarkDeliveryCompletedParams{ID: del.ID, ClaimToken: del.ClaimToken}); err != nil {
			d.log.WithError(err).Error("marking webhook delivery completed")
		}
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
		attemptID := uuid.Nil
		if attempt != nil {
			attemptID = attempt.ID
		}
		if err := q.RequeueDelivery(ctx, webhooksql.RequeueDeliveryParams{
			ID:         del.ID,
			ClaimToken: del.ClaimToken,
			AttemptID:  attemptID,
			RetryCount: int32(nextRetry),
			RunAt:      pgtype.Timestamptz{Time: runAt, Valid: true},
		}); err != nil {
			d.log.WithError(err).Error("requeueing webhook delivery")
		}
		d.metrics.processedCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "requeued"),
		))
	} else {
		if err := q.MarkDeliveryFailed(ctx, webhooksql.MarkDeliveryFailedParams{ID: del.ID, ClaimToken: del.ClaimToken}); err != nil {
			d.log.WithError(err).Error("marking webhook delivery as failed")
		}
		d.metrics.processedCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "failed"),
		))
	}
}

// deliver sends a single HTTP delivery attempt to sub and records it in the audit log.
// deliveryRowID links the audit row back to the originating webhook_event_deliveries row,
// and is nil for manual tests, which never change subscription health.
func (d *Dispatcher) deliver(ctx context.Context, q webhooksql.Querier, sub *webhooksql.WebhookSubscription, eventType string, payload []byte, deliveryRowID *uuid.UUID) (*webhooksql.WebhookDelivery, error) {
	signature := SignPayload(sub.Secret, payload)

	start := time.Now()
	var (
		responseStatus        *int32
		responseBody          []byte
		errorMessage          *string
		responseBodyTruncated bool
		requestHeaders        = http.Header{}
		responseHeaders       = http.Header{}
		success               bool
	)
	statusStr := "network_error"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Url, bytes.NewReader(payload))
	if err != nil {
		errorMessage = new("Invalid webhook endpoint URL.")
	} else {
		req.Header.Set("Content-Type", contentTypeHeader)
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("User-Agent", userAgentHeader)
		req.Header.Set(signatureHeader, signature)
		requestHeaders = req.Header.Clone()
		requestHeaders.Set("Content-Length", strconv.FormatInt(req.ContentLength, 10))
		resp, err := d.httpClient.Do(req)
		if err != nil {
			errorMessage = new("The HTTP request failed before a response could be received.")
		} else {
			defer resp.Body.Close()
			status := int32(resp.StatusCode)
			responseStatus = &status
			responseHeaders = resp.Header
			statusStr = strconv.Itoa(resp.StatusCode)
			success = resp.StatusCode >= 200 && resp.StatusCode < 300
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
			responseBodyTruncated = len(body) > maxResponseBodySize
			responseBody = body[:min(len(body), maxResponseBodySize)]
			if responseBody == nil {
				responseBody = []byte{}
			}
			if readErr != nil {
				errorMessage = new("The response body could not be read completely.")
				success = false
			}
		}
	}
	durationSeconds := time.Since(start).Seconds()
	durationMs := int32(durationSeconds * 1000)

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
	attempt, recordErr := q.CreateDelivery(ctx, webhooksql.CreateDeliveryParams{
		SubscriptionID:         sub.ID,
		WebhookEventDeliveryID: deliveryRowID,
		EventType:              eventType,
		RequestBody:            payload,
		RequestHeaders:         storedRequestHeaders(requestHeaders),
		ResponseStatus:         responseStatus,
		ResponseBody:           responseBody,
		ResponseHeaders:        storedResponseHeaders(responseHeaders),
		ResponseBodyTruncated:  responseBodyTruncated,
		ErrorMessage:           errorMessage,
		DurationMs:             durationMs,
		Success:                success,
	})
	if recordErr != nil {
		return nil, recordErr
	}
	if deliveryRowID == nil {
		return attempt, nil
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

	return attempt, nil
}

func storedRequestHeaders(headers http.Header) []byte {
	safe := make(http.Header)
	for _, name := range []string{
		"Accept", "Accept-Encoding", "Content-Type", "Content-Length", "Content-Encoding",
		"User-Agent", "Date", "Retry-After", "Server", "X-Request-Id", "Traceparent",
		signatureHeader,
	} {
		if values := headers.Values(name); len(values) > 0 {
			safe[http.CanonicalHeaderKey(name)] = values
		}
	}
	data, _ := json.Marshal(safe)
	return data
}

func storedResponseHeaders(headers http.Header) []byte {
	safe := make(http.Header)
	for name, values := range headers {
		name = http.CanonicalHeaderKey(name)
		switch name {
		case "Authorization", "Proxy-Authorization",
			"Www-Authenticate", "Proxy-Authenticate",
			"Authentication-Info", "Proxy-Authentication-Info",
			"Cookie", "Cookie2", "Set-Cookie", "Set-Cookie2",
			"Api-Key", "X-Api-Key", "X-Auth-Token", "X-Access-Token",
			"X-Csrf-Token", "X-Csrftoken", "X-Xsrf-Token":
			continue
		}
		safe[name] = append(safe[name], values...)
	}
	data, _ := json.Marshal(safe)
	return data
}

// SerializeEventData is a helper to serialize event data to JSON for the webhook payload.
func SerializeEventData(data any) ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	return json.Marshal(data)
}

func (d *Dispatcher) SendTest(ctx context.Context, q webhooksql.Querier, sub *webhooksql.WebhookSubscription, actor string) (*webhooksql.WebhookDelivery, error) {
	testEvent := WebhookEvent{
		RawEventType: "WEBHOOK_TEST",
		TeamSlug:     sub.TeamSlug,
		Actor:        actor,
		ResourceType: string(ActivityLogEntryResourceTypeWebhook),
		ResourceName: sub.ID.String(),
		TriggeredAt:  time.Now().UTC(),
		Test:         true,
	}

	payload, err := BuildCloudEvent(d.source, uuid.New().String(), testEvent)
	if err != nil {
		return nil, fmt.Errorf("building test CloudEvent: %w", err)
	}

	return d.deliver(ctx, q, sub, "WEBHOOK_TEST", payload, nil)
}
