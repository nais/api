//go:build integration_test

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/notify"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	testActivityType = activitylog.ActivityLogActivityType("WEBHOOK_INTEGRATION_TEST_EVENT")
	testResourceType = "WEBHOOK_INTEGRATION_TEST_RESOURCE"
)

func init() {
	activitylog.RegisterActivityType(testActivityType, activitylog.ActivityLogEntryActionCreated, testResourceType)
}

func TestDispatcherIntegration(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("fan out and deliver success", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		createTeam(ctx, t, pool, "team-a")
		team := slug.Slug("team-a")

		srv := newRecordingServer(http.StatusOK)
		defer srv.Close()

		q := webhooksql.New(pool)
		sub, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        srv.URL,
			Secret:     "test-secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create subscription: %v", err)
		}

		insertActivityLogEntry(ctx, t, pool, &team, "my-resource")

		d := newTestDispatcher(t, pool, log)
		d.fanOutPendingEvents(ctx)
		d.drainPendingDeliveries(ctx)

		if got := srv.count(); got != 1 {
			t.Fatalf("expected exactly 1 request, got %d", got)
		}

		req := srv.requestAt(0)
		if req.signature == "" {
			t.Error("expected X-Webhook-Signature header to be set")
		}

		var ce CloudEvent
		if err := json.Unmarshal(req.body, &ce); err != nil {
			t.Fatalf("failed to unmarshal delivered CloudEvent: %v", err)
		}
		wantSubject := "team-a/my-resource"
		if ce.Subject != wantSubject {
			t.Errorf("Subject = %q, want %q", ce.Subject, wantSubject)
		}
		wantType := activitylog.CloudEventType(testActivityType)
		if ce.Type != wantType {
			t.Errorf("Type = %q, want %q", ce.Type, wantType)
		}

		assertOnlyOutboxEventStatus(ctx, t, pool, "completed")

		delivery := fetchOnlyDeliveryForSubscription(ctx, t, pool, sub.ID)
		if delivery.Status != webhooksql.WebhookDeliveryStatusCompleted {
			t.Errorf("delivery status = %q, want %q", delivery.Status, webhooksql.WebhookDeliveryStatusCompleted)
		}
		if ce.ID != delivery.ID.String() {
			t.Errorf("CloudEvent id = %q, want delivery id %q", ce.ID, delivery.ID.String())
		}

		auditRows := listAuditDeliveries(ctx, t, pool, sub.ID)
		if len(auditRows) != 1 {
			t.Fatalf("expected exactly 1 audit delivery row, got %d", len(auditRows))
		}
		if !auditRows[0].Success {
			t.Error("expected audit delivery row to record success=true")
		}

		updatedSub, err := q.GetSubscription(ctx, sub.ID)
		if err != nil {
			t.Fatalf("failed to fetch subscription: %v", err)
		}
		if updatedSub.ConsecutiveFailures != 0 {
			t.Errorf("ConsecutiveFailures = %d, want 0", updatedSub.ConsecutiveFailures)
		}
	})

	t.Run("fan out is idempotent", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		srv := newRecordingServer(http.StatusOK)
		defer srv.Close()

		q := webhooksql.New(pool)
		sub, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        srv.URL,
			Secret:     "test-secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create subscription: %v", err)
		}

		insertActivityLogEntry(ctx, t, pool, nil, "my-resource")
		eventID := fetchOnlyOutboxEventID(ctx, t, pool)

		params := webhooksql.CreateEventDeliveryParams{
			WebhookEventID: eventID,
			SubscriptionID: sub.ID,
		}
		if err := q.CreateEventDelivery(ctx, params); err != nil {
			t.Fatalf("first CreateEventDelivery failed: %v", err)
		}
		if err := q.CreateEventDelivery(ctx, params); err != nil {
			t.Fatalf("second CreateEventDelivery failed: %v", err)
		}

		var count int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_event_deliveries WHERE webhook_event_id = $1 AND subscription_id = $2`, eventID, sub.ID).Scan(&count); err != nil {
			t.Fatalf("failed to count delivery rows: %v", err)
		}
		if count != 1 {
			t.Errorf("expected exactly 1 delivery row after two CreateEventDelivery calls, got %d", count)
		}
	})

	t.Run("delivery retry and backoff", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		srv := newRecordingServer(http.StatusInternalServerError)
		defer srv.Close()

		q := webhooksql.New(pool)
		sub, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        srv.URL,
			Secret:     "test-secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create subscription: %v", err)
		}

		insertActivityLogEntry(ctx, t, pool, nil, "my-resource")

		d := newTestDispatcher(t, pool, log)
		d.fanOutPendingEvents(ctx)
		d.drainPendingDeliveries(ctx)

		delivery := fetchOnlyDeliveryForSubscription(ctx, t, pool, sub.ID)
		if delivery.Status != webhooksql.WebhookDeliveryStatusPending {
			t.Errorf("delivery status = %q, want %q", delivery.Status, webhooksql.WebhookDeliveryStatusPending)
		}
		if delivery.RetryCount != 1 {
			t.Errorf("RetryCount = %d, want 1", delivery.RetryCount)
		}

		wantRunAt := time.Now().Add(retryBackoffs[0])
		if diff := delivery.RunAt.Time.Sub(wantRunAt); diff < -10*time.Second || diff > 10*time.Second {
			t.Errorf("RunAt = %v, want approximately %v (diff %v)", delivery.RunAt.Time, wantRunAt, diff)
		}

		updatedSub, err := q.GetSubscription(ctx, sub.ID)
		if err != nil {
			t.Fatalf("failed to fetch subscription: %v", err)
		}
		if updatedSub.ConsecutiveFailures != 1 {
			t.Errorf("ConsecutiveFailures = %d, want 1", updatedSub.ConsecutiveFailures)
		}

		auditRows := listAuditDeliveries(ctx, t, pool, sub.ID)
		if len(auditRows) != 1 {
			t.Fatalf("expected exactly 1 audit delivery row, got %d", len(auditRows))
		}
		if auditRows[0].Success {
			t.Error("expected audit delivery row to record success=false")
		}
		if auditRows[0].ResponseStatus == nil || *auditRows[0].ResponseStatus != http.StatusInternalServerError {
			t.Errorf("ResponseStatus = %v, want %d", auditRows[0].ResponseStatus, http.StatusInternalServerError)
		}
	})

	t.Run("delivery reaches failed status after exhausting retry budget", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		srv := newRecordingServer(http.StatusInternalServerError)
		defer srv.Close()

		q := webhooksql.New(pool)
		sub, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        srv.URL,
			Secret:     "test-secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create subscription: %v", err)
		}

		insertActivityLogEntry(ctx, t, pool, nil, "my-resource")

		d := newTestDispatcher(t, pool, log)
		d.fanOutPendingEvents(ctx)

		rows, err := q.ClaimPendingDeliveries(ctx, 1)
		if err != nil {
			t.Fatalf("failed to claim delivery: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected exactly 1 claimable delivery, got %d", len(rows))
		}
		del, subRow, entry := rows[0].WebhookEventDelivery, rows[0].WebhookSubscription, rows[0].ActivityLogEntry

		// Drive the same delivery through every retry attempt directly, bypassing the
		// run_at gate that ClaimPendingDeliveries would otherwise enforce, so the test
		// doesn't need to wait on real backoff durations.
		for i := 0; i <= maxRetryCount; i++ {
			d.processDelivery(ctx, q, &del, &subRow, &entry)
			del = fetchDeliveryByID(ctx, t, pool, del.ID)
		}

		if del.Status != webhooksql.WebhookDeliveryStatusFailed {
			t.Errorf("delivery status = %q, want %q after %d attempts", del.Status, webhooksql.WebhookDeliveryStatusFailed, maxRetryCount+1)
		}

		auditRows := listAuditDeliveries(ctx, t, pool, sub.ID)
		if len(auditRows) != maxRetryCount+1 {
			t.Errorf("expected %d audit delivery rows, got %d", maxRetryCount+1, len(auditRows))
		}
	})

	t.Run("subscription auto-disables after repeated failures", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		srv := newRecordingServer(http.StatusInternalServerError)
		defer srv.Close()

		q := webhooksql.New(pool)
		sub, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        srv.URL,
			Secret:     "test-secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create subscription: %v", err)
		}

		// consecutive_failures increments once per failed delivery attempt across any
		// event, so disableThreshold separate events is the cleanest way to reach it
		// deterministically without looping retries on a single delivery.
		for i := 0; i < disableThreshold; i++ {
			insertActivityLogEntry(ctx, t, pool, nil, fmt.Sprintf("resource-%d", i))
		}

		d := newTestDispatcher(t, pool, log)
		d.fanOutPendingEvents(ctx)
		d.drainPendingDeliveries(ctx)

		updatedSub, err := q.GetSubscription(ctx, sub.ID)
		if err != nil {
			t.Fatalf("failed to fetch subscription: %v", err)
		}
		if updatedSub.Enabled {
			t.Error("expected subscription to be auto-disabled")
		}
		if !updatedSub.DisabledAt.Valid {
			t.Error("expected DisabledAt to be set")
		}
	})

	t.Run("per-subscriber isolation", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		healthySrv := newRecordingServer(http.StatusOK)
		defer healthySrv.Close()
		failingSrv := newRecordingServer(http.StatusInternalServerError)
		defer failingSrv.Close()

		q := webhooksql.New(pool)
		healthySub, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        healthySrv.URL,
			Secret:     "secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create healthy subscription: %v", err)
		}
		failingSub, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        failingSrv.URL,
			Secret:     "secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create failing subscription: %v", err)
		}

		insertActivityLogEntry(ctx, t, pool, nil, "shared-resource")

		d := newTestDispatcher(t, pool, log)
		d.fanOutPendingEvents(ctx)
		d.drainPendingDeliveries(ctx)

		healthyDelivery := fetchOnlyDeliveryForSubscription(ctx, t, pool, healthySub.ID)
		if healthyDelivery.Status != webhooksql.WebhookDeliveryStatusCompleted {
			t.Errorf("healthy subscriber delivery status = %q, want %q", healthyDelivery.Status, webhooksql.WebhookDeliveryStatusCompleted)
		}

		failingDelivery := fetchOnlyDeliveryForSubscription(ctx, t, pool, failingSub.ID)
		if failingDelivery.Status != webhooksql.WebhookDeliveryStatusPending {
			t.Errorf("failing subscriber delivery status = %q, want %q", failingDelivery.Status, webhooksql.WebhookDeliveryStatusPending)
		}
		if failingDelivery.RetryCount != 1 {
			t.Errorf("failing subscriber RetryCount = %d, want 1", failingDelivery.RetryCount)
		}
	})

	t.Run("stable CloudEvent id across retries", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		var attempts atomic.Int32
		var mu sync.Mutex
		var bodies [][]byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			bodies = append(bodies, body)
			mu.Unlock()

			if attempts.Add(1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		q := webhooksql.New(pool)
		if _, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        srv.URL,
			Secret:     "secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		}); err != nil {
			t.Fatalf("failed to create subscription: %v", err)
		}

		insertActivityLogEntry(ctx, t, pool, nil, "my-resource")

		d := newTestDispatcher(t, pool, log)
		d.fanOutPendingEvents(ctx)

		rows, err := q.ClaimPendingDeliveries(ctx, 1)
		if err != nil {
			t.Fatalf("failed to claim delivery: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected exactly 1 claimable delivery, got %d", len(rows))
		}
		del, subRow, entry := rows[0].WebhookEventDelivery, rows[0].WebhookSubscription, rows[0].ActivityLogEntry

		// First attempt fails and is requeued.
		d.processDelivery(ctx, q, &del, &subRow, &entry)
		del = fetchDeliveryByID(ctx, t, pool, del.ID)
		if del.Status != webhooksql.WebhookDeliveryStatusPending {
			t.Fatalf("expected delivery to be requeued after first failed attempt, status = %q", del.Status)
		}

		// Simulate the backoff having elapsed, then let the delivery be claimed and
		// processed again through the normal drain path (rather than calling
		// processDelivery directly), since ClaimPendingDeliveries is what optimistically
		// marks a delivery completed at claim time.
		if _, err := pool.Exec(ctx, `UPDATE webhook_event_deliveries SET run_at = NOW() WHERE id = $1`, del.ID); err != nil {
			t.Fatalf("failed to fast-forward run_at: %v", err)
		}
		d.drainPendingDeliveries(ctx)
		del = fetchDeliveryByID(ctx, t, pool, del.ID)
		if del.Status != webhooksql.WebhookDeliveryStatusCompleted {
			t.Fatalf("expected delivery to be completed after second attempt, status = %q", del.Status)
		}

		mu.Lock()
		gotBodies := append([][]byte(nil), bodies...)
		mu.Unlock()
		if len(gotBodies) != 2 {
			t.Fatalf("expected exactly 2 requests, got %d", len(gotBodies))
		}

		var firstCE, secondCE CloudEvent
		if err := json.Unmarshal(gotBodies[0], &firstCE); err != nil {
			t.Fatalf("failed to unmarshal first request body: %v", err)
		}
		if err := json.Unmarshal(gotBodies[1], &secondCE); err != nil {
			t.Fatalf("failed to unmarshal second request body: %v", err)
		}

		if firstCE.ID != secondCE.ID {
			t.Errorf("CloudEvent id changed across retries: %q != %q", firstCE.ID, secondCE.ID)
		}
		if firstCE.ID != del.ID.String() {
			t.Errorf("CloudEvent id = %q, want delivery id %q", firstCE.ID, del.ID.String())
		}
	})

	t.Run("ping", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		srv := newRecordingServer(http.StatusOK)
		defer srv.Close()

		q := webhooksql.New(pool)
		row, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        srv.URL,
			Secret:     "secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create subscription: %v", err)
		}

		d := newTestDispatcher(t, pool, log)
		gsub := toGraphSubscription(row)
		if err := d.Ping(ctx, gsub); err != nil {
			t.Fatalf("Ping() error = %v", err)
		}

		if got := srv.count(); got != 1 {
			t.Fatalf("expected exactly 1 request, got %d", got)
		}

		var ce CloudEvent
		if err := json.Unmarshal(srv.requestAt(0).body, &ce); err != nil {
			t.Fatalf("failed to unmarshal ping CloudEvent: %v", err)
		}
		if ce.Type != "io.nais.ping" {
			t.Errorf("Type = %q, want %q", ce.Type, "io.nais.ping")
		}

		var deliveryID *uuid.UUID
		var eventType string
		var success bool
		err = pool.QueryRow(ctx, `SELECT webhook_event_delivery_id, event_type, success FROM webhook_deliveries WHERE subscription_id = $1`, row.ID).
			Scan(&deliveryID, &eventType, &success)
		if err != nil {
			t.Fatalf("failed to fetch ping audit row: %v", err)
		}
		if deliveryID != nil {
			t.Errorf("webhook_event_delivery_id = %v, want nil", deliveryID)
		}
		if eventType != "ping" {
			t.Errorf("event_type = %q, want %q", eventType, "ping")
		}
		if !success {
			t.Error("expected ping delivery to be recorded as successful")
		}
	})

	t.Run("prune queries", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		q := webhooksql.New(pool)

		sub, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        "https://example.invalid",
			Secret:     "secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create subscription: %v", err)
		}
		sub2, err := q.CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			Url:        "https://example.invalid/2",
			Secret:     "secret",
			EventTypes: []string{"*"},
			CreatedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("failed to create second subscription: %v", err)
		}

		// Only used to obtain a valid activity_log_entries row to satisfy the FK on
		// webhook_events; the auto-created outbox row is discarded so the rest of this
		// test can fully control the rows it exercises.
		insertActivityLogEntry(ctx, t, pool, nil, "resource-a")
		activityLogEntryID := fetchOnlyActivityLogEntryID(ctx, t, pool)
		if _, err := pool.Exec(ctx, `DELETE FROM webhook_events`); err != nil {
			t.Fatalf("failed to clear auto-created outbox event: %v", err)
		}

		prunableEventID := createOutboxEventDirect(ctx, t, pool, activityLogEntryID, webhooksql.WebhookOutboxStatusCompleted, 10*24*time.Hour)
		eventWithPendingChildID := createOutboxEventDirect(ctx, t, pool, activityLogEntryID, webhooksql.WebhookOutboxStatusCompleted, 10*24*time.Hour)
		freshEventID := createOutboxEventDirect(ctx, t, pool, activityLogEntryID, webhooksql.WebhookOutboxStatusCompleted, 0)

		pendingChildDeliveryID := createEventDeliveryDirect(ctx, t, pool, eventWithPendingChildID, sub.ID, webhooksql.WebhookDeliveryStatusPending, 10*24*time.Hour)

		before := pgtypeTimestamptz(-7 * 24 * time.Hour)
		if err := q.PruneOldOutboxEvents(ctx, before); err != nil {
			t.Fatalf("PruneOldOutboxEvents() error = %v", err)
		}

		remaining := fetchOutboxEventIDs(ctx, t, pool)
		assertSameUUIDSet(t, remaining, []uuid.UUID{eventWithPendingChildID, freshEventID})
		if containsUUID(remaining, prunableEventID) {
			t.Error("expected the old, childless, completed outbox event to be pruned")
		}

		// Event deliveries: an old completed row should be pruned; the old pending row
		// from above should survive (status filter), and a fresh completed row should
		// also survive (age filter).
		oldCompletedDeliveryID := createEventDeliveryDirect(ctx, t, pool, freshEventID, sub.ID, webhooksql.WebhookDeliveryStatusCompleted, 10*24*time.Hour)
		freshCompletedDeliveryID := createEventDeliveryDirect(ctx, t, pool, freshEventID, sub2.ID, webhooksql.WebhookDeliveryStatusCompleted, 0)

		if err := q.PruneOldEventDeliveries(ctx, before); err != nil {
			t.Fatalf("PruneOldEventDeliveries() error = %v", err)
		}

		if deliveryExists(ctx, t, pool, oldCompletedDeliveryID) {
			t.Error("expected old completed delivery to be pruned")
		}
		if !deliveryExists(ctx, t, pool, pendingChildDeliveryID) {
			t.Error("expected old pending delivery to survive pruning (status filter)")
		}
		if !deliveryExists(ctx, t, pool, freshCompletedDeliveryID) {
			t.Error("expected fresh completed delivery to survive pruning (age filter)")
		}

		// Delivery audit log.
		oldAuditID := createAuditDeliveryDirect(ctx, t, pool, sub.ID, 40*24*time.Hour)
		freshAuditID := createAuditDeliveryDirect(ctx, t, pool, sub.ID, 0)

		deliveryBefore := pgtypeTimestamptz(-30 * 24 * time.Hour)
		if err := q.PruneDeliveries(ctx, deliveryBefore); err != nil {
			t.Fatalf("PruneDeliveries() error = %v", err)
		}

		if auditDeliveryExists(ctx, t, pool, oldAuditID) {
			t.Error("expected old audit delivery row to be pruned")
		}
		if !auditDeliveryExists(ctx, t, pool, freshAuditID) {
			t.Error("expected fresh audit delivery row to survive pruning")
		}
	})
}

func newTestDispatcher(t *testing.T, pool *pgxpool.Pool, log logrus.FieldLogger) *Dispatcher {
	t.Helper()
	d, err := NewDispatcher(pool, notify.New(pool, log), "https://test.example", log)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}
	return d
}

func createTeam(ctx context.Context, t *testing.T, pool *pgxpool.Pool, teamSlug string) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO teams (slug, purpose, slack_channel) VALUES ($1, 'test team', '#test')`, teamSlug)
	if err != nil {
		t.Fatalf("failed to create team %q: %v", teamSlug, err)
	}
}

func insertActivityLogEntry(ctx context.Context, t *testing.T, pool *pgxpool.Pool, teamSlug *slug.Slug, resourceName string) {
	t.Helper()
	err := activitylogsql.New(pool).Create(ctx, activitylogsql.CreateParams{
		Actor:        "actor@example.com",
		Action:       string(activitylog.ActivityLogEntryActionCreated),
		ResourceType: testResourceType,
		ResourceName: resourceName,
		TeamSlug:     teamSlug,
	})
	if err != nil {
		t.Fatalf("failed to insert activity log entry: %v", err)
	}
}

type recordedRequest struct {
	signature string
	body      []byte
}

// recordingServer is an httptest.Server that always responds with a fixed status code, and
// remembers every request it received so tests can assert on delivered payloads/headers.
type recordingServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []recordedRequest
}

func newRecordingServer(status int) *recordingServer {
	rs := &recordingServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rs.mu.Lock()
		rs.requests = append(rs.requests, recordedRequest{signature: r.Header.Get(signatureHeader), body: body})
		rs.mu.Unlock()
		w.WriteHeader(status)
	}))
	return rs
}

func (rs *recordingServer) count() int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return len(rs.requests)
}

func (rs *recordingServer) requestAt(i int) recordedRequest {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.requests[i]
}

func assertOnlyOutboxEventStatus(ctx context.Context, t *testing.T, pool *pgxpool.Pool, want string) {
	t.Helper()
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM webhook_events`).Scan(&status); err != nil {
		t.Fatalf("failed to fetch outbox event status: %v", err)
	}
	if status != want {
		t.Errorf("outbox event status = %q, want %q", status, want)
	}
}

func fetchOnlyOutboxEventID(ctx context.Context, t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM webhook_events`).Scan(&id); err != nil {
		t.Fatalf("failed to fetch outbox event id: %v", err)
	}
	return id
}

func fetchOnlyActivityLogEntryID(ctx context.Context, t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM activity_log_entries`).Scan(&id); err != nil {
		t.Fatalf("failed to fetch activity log entry id: %v", err)
	}
	return id
}

func fetchOutboxEventIDs(ctx context.Context, t *testing.T, pool *pgxpool.Pool) []uuid.UUID {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT id FROM webhook_events ORDER BY created_at ASC`)
	if err != nil {
		t.Fatalf("failed to fetch outbox event ids: %v", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("failed to scan outbox event id: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func createOutboxEventDirect(ctx context.Context, t *testing.T, pool *pgxpool.Pool, activityLogEntryID uuid.UUID, status webhooksql.WebhookOutboxStatus, age time.Duration) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO webhook_events (activity_log_entries_id, status, created_at)
		VALUES ($1, $2, NOW() - $3::INTERVAL)
		RETURNING id
	`, activityLogEntryID, string(status), age.String()).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert outbox event directly: %v", err)
	}
	return id
}

func createEventDeliveryDirect(ctx context.Context, t *testing.T, pool *pgxpool.Pool, eventID, subscriptionID uuid.UUID, status webhooksql.WebhookDeliveryStatus, age time.Duration) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO webhook_event_deliveries (webhook_event_id, subscription_id, status, created_at)
		VALUES ($1, $2, $3, NOW() - $4::INTERVAL)
		RETURNING id
	`, eventID, subscriptionID, string(status), age.String()).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert event delivery directly: %v", err)
	}
	return id
}

func deliveryExists(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id uuid.UUID) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhook_event_deliveries WHERE id = $1)`, id).Scan(&exists); err != nil {
		t.Fatalf("failed to check delivery existence: %v", err)
	}
	return exists
}

func createAuditDeliveryDirect(ctx context.Context, t *testing.T, pool *pgxpool.Pool, subscriptionID uuid.UUID, age time.Duration) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO webhook_deliveries (subscription_id, event_type, request_body, duration_ms, success, created_at)
		VALUES ($1, 'test', '{}'::jsonb, 1, true, NOW() - $2::INTERVAL)
		RETURNING id
	`, subscriptionID, age.String()).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert audit delivery directly: %v", err)
	}
	return id
}

func auditDeliveryExists(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id uuid.UUID) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhook_deliveries WHERE id = $1)`, id).Scan(&exists); err != nil {
		t.Fatalf("failed to check audit delivery existence: %v", err)
	}
	return exists
}

func fetchDeliveryByID(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id uuid.UUID) webhooksql.WebhookEventDelivery {
	t.Helper()
	var d webhooksql.WebhookEventDelivery
	err := pool.QueryRow(ctx, `
		SELECT id, webhook_event_id, subscription_id, status, retry_count, run_at, created_at
		FROM webhook_event_deliveries WHERE id = $1
	`, id).Scan(&d.ID, &d.WebhookEventID, &d.SubscriptionID, &d.Status, &d.RetryCount, &d.RunAt, &d.CreatedAt)
	if err != nil {
		t.Fatalf("failed to fetch delivery row: %v", err)
	}
	return d
}

func fetchOnlyDeliveryForSubscription(ctx context.Context, t *testing.T, pool *pgxpool.Pool, subscriptionID uuid.UUID) webhooksql.WebhookEventDelivery {
	t.Helper()
	var d webhooksql.WebhookEventDelivery
	err := pool.QueryRow(ctx, `
		SELECT id, webhook_event_id, subscription_id, status, retry_count, run_at, created_at
		FROM webhook_event_deliveries WHERE subscription_id = $1
	`, subscriptionID).Scan(&d.ID, &d.WebhookEventID, &d.SubscriptionID, &d.Status, &d.RetryCount, &d.RunAt, &d.CreatedAt)
	if err != nil {
		t.Fatalf("failed to fetch delivery row for subscription %s: %v", subscriptionID, err)
	}
	return d
}

func listAuditDeliveries(ctx context.Context, t *testing.T, pool *pgxpool.Pool, subscriptionID uuid.UUID) []webhooksql.WebhookDelivery {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT id, subscription_id, webhook_event_delivery_id, event_type, request_body, response_status, response_body, duration_ms, success, created_at
		FROM webhook_deliveries WHERE subscription_id = $1 ORDER BY created_at ASC
	`, subscriptionID)
	if err != nil {
		t.Fatalf("failed to list audit deliveries: %v", err)
	}
	defer rows.Close()

	var result []webhooksql.WebhookDelivery
	for rows.Next() {
		var d webhooksql.WebhookDelivery
		if err := rows.Scan(&d.ID, &d.SubscriptionID, &d.WebhookEventDeliveryID, &d.EventType, &d.RequestBody, &d.ResponseStatus, &d.ResponseBody, &d.DurationMs, &d.Success, &d.CreatedAt); err != nil {
			t.Fatalf("failed to scan audit delivery: %v", err)
		}
		result = append(result, d)
	}
	return result
}

func containsUUID(haystack []uuid.UUID, needle uuid.UUID) bool {
	for _, id := range haystack {
		if id == needle {
			return true
		}
	}
	return false
}

// assertSameUUIDSet compares got and want as sets (order-independent), using go-cmp for a
// readable diff on failure.
func assertSameUUIDSet(t *testing.T, got, want []uuid.UUID) {
	t.Helper()

	toSet := func(ids []uuid.UUID) map[uuid.UUID]bool {
		set := make(map[uuid.UUID]bool, len(ids))
		for _, id := range ids {
			set[id] = true
		}
		return set
	}

	if diff := cmp.Diff(toSet(want), toSet(got)); diff != "" {
		t.Errorf("unexpected set of ids (-want +got):\n%s", diff)
	}
}

func pgtypeTimestamptz(offset time.Duration) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now().Add(offset), Valid: true}
}

func startPostgresql(ctx context.Context, t *testing.T, log logrus.FieldLogger) (container *postgres.PostgresContainer, dsn string, err error) {
	container, err = postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	defer testcontainers.CleanupContainer(t, container)

	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	dsn, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	pool, err := database.NewPool(ctx, dsn, log, true)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create pool: %w", err)
	}
	pool.Close()

	if err := container.Snapshot(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, dsn, nil
}

func getConnection(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string, log logrus.FieldLogger) *pgxpool.Pool {
	pool, _ := database.NewPool(ctx, dsn, log, false)

	t.Cleanup(func() {
		pool.Close()
		if err := container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
	})

	return pool
}
