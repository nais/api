package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	logrustest "github.com/sirupsen/logrus/hooks/test"
)

func TestDeliverPreservesDiagnosticsAndDoesNotChangeTestHealth(t *testing.T) {
	for _, tt := range []struct {
		name      string
		body      []byte
		status    int
		truncated bool
	}{
		{name: "binary response", body: []byte{0xff, 0x00, 0xfe}, status: http.StatusOK},
		{name: "failed response", body: []byte("failed"), status: http.StatusInternalServerError},
		{name: "truncated response", body: bytes.Repeat([]byte("x"), maxResponseBodySize+1), status: http.StatusOK, truncated: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			q := newWorkerTestQueries(0)
			payload := []byte("{\n  \"test\": true\n}")
			d := newWorkerTestDispatcher(t, q, func(req *http.Request) (*http.Response, error) {
				if got, want := req.Header.Get(signatureHeader), SignPayload("test-secret", payload); got != want {
					t.Errorf("signature = %q, want %q", got, want)
				}
				response := workerTestResponse(tt.status)
				response.Body = io.NopCloser(bytes.NewReader(tt.body))
				response.Header.Add("X-Request-Id", "one")
				response.Header.Add("X-Request-Id", "two")
				response.Header.Set("X-Service-Version", "v2")
				response.Header.Set("Set-Cookie", "sensitive")
				response.Header.Set("X-Api-Key", "sensitive")
				return response, nil
			})
			sub := &webhooksql.WebhookSubscription{ID: uuid.New(), Url: "https://example.test", Secret: "test-secret"}
			attempt, err := d.deliver(t.Context(), q, sub, "WEBHOOK_TEST", payload, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(attempt.RequestBody, payload) || !bytes.Equal(attempt.ResponseBody, tt.body[:min(len(tt.body), maxResponseBodySize)]) {
				t.Error("recorded body bytes differ from the request or response")
			}
			if attempt.ResponseBodyTruncated != tt.truncated || attempt.Success != (tt.status == http.StatusOK) {
				t.Errorf("unexpected attempt result: %+v", attempt)
			}
			var headers http.Header
			if err := json.Unmarshal(attempt.ResponseHeaders, &headers); err != nil {
				t.Fatal(err)
			}
			if values := headers.Values("X-Request-Id"); len(values) != 2 || values[0] != "one" || values[1] != "two" {
				t.Errorf("multi-valued headers were not preserved: %v", headers)
			}
			if len(headers) != 2 || headers.Get("X-Service-Version") != "v2" {
				t.Errorf("unexpected recorded response headers: %v", headers)
			}
			if len(q.results) != 0 || len(q.requeued) != 0 {
				t.Fatal("manual test changed subscription health or scheduled a retry")
			}
		})
	}
}

func TestStoredRequestHeadersOmitsCredentials(t *testing.T) {
	headers := make(http.Header)
	for _, name := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "X-Api-Key"} {
		headers.Set(name, "sensitive")
	}
	headers.Set("Content-Type", "application/json")
	var got http.Header
	if err := json.Unmarshal(storedRequestHeaders(headers), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got.Get("Content-Type") != "application/json" {
		t.Errorf("unexpected stored headers: %v", got)
	}
}

func TestStoredResponseHeaders(t *testing.T) {
	headers := http.Header{
		"Content-Type":    {"application/json"},
		"x-service-debug": {"one", "two"},
	}
	for _, name := range []string{
		"Authorization", "Proxy-Authorization", "WWW-Authenticate", "Proxy-Authenticate",
		"Authentication-Info", "Proxy-Authentication-Info",
		"Cookie", "Cookie2", "Set-Cookie", "Set-Cookie2",
		"Api-Key", "X-Api-Key", "X-Auth-Token", "X-Access-Token",
		"X-CSRF-Token", "X-CSRFToken", "X-XSRF-Token",
	} {
		headers[strings.ToLower(name)] = []string{"sensitive"}
		headers[strings.ToUpper(name)] = []string{"sensitive"}
	}
	var got http.Header
	if err := json.Unmarshal(storedResponseHeaders(headers), &got); err != nil {
		t.Fatal(err)
	}
	want := http.Header{
		"Content-Type":    {"application/json"},
		"X-Service-Debug": {"one", "two"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected stored response headers (-want +got):\n%s", diff)
	}
}

func TestProcessDelivery_SkipsNewlyIgnoredQueuedEvent(t *testing.T) {
	const activityType = "WEBHOOK_TEST_QUEUED_IGNORED_EVENT"
	const resourceType = "WEBHOOK_TEST_QUEUED_IGNORED_RESOURCE"
	activitylog.RegisterActivityType(activityType, activitylog.ActivityLogEntryActionCreated, resourceType, "Test activity.")
	entry := &webhooksql.ActivityLogEntry{
		ResourceType: resourceType,
		Action:       string(activitylog.ActivityLogEntryActionCreated),
	}
	activitylog.RegisterActivityType(activityType, activitylog.ActivityLogEntryActionCreated, resourceType, "Test activity.", activitylog.IgnoreWebhook())

	for _, allEvents := range []bool{true, false} {
		sub := &webhooksql.WebhookSubscription{AllEvents: allEvents, Enabled: true}
		if !allEvents {
			sub.EventTypes = []string{activityType}
		}
		for _, retryCount := range []int32{0, 1} {
			d := &Dispatcher{}
			// Ignored deliveries only finish queue bookkeeping, without HTTP or metrics.
			d.processDelivery(t.Context(), newWorkerTestQueries(0),
				&webhooksql.WebhookEventDelivery{RetryCount: retryCount},
				sub,
				entry,
			)
		}
	}
}

func TestDeliveryWorkersLimitConcurrencyAndFinishClaimedWork(t *testing.T) {
	if deliveryWorkers != 3 {
		t.Fatalf("deliveryWorkers = %d, want 3", deliveryWorkers)
	}
	q := newWorkerTestQueries(6)
	started := make(chan struct{}, 6)
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	var active atomic.Int32
	d := newWorkerTestDispatcher(t, q, func(req *http.Request) (*http.Response, error) {
		if n := active.Add(1); n > 3 {
			t.Errorf("active deliveries = %d, maximum is 3", n)
		}
		defer active.Add(-1)
		started <- struct{}{}
		select {
		case <-release:
			return workerTestResponse(http.StatusOK), nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	})
	cancel, done := startTestDeliveryWorkers(t, d)
	t.Cleanup(unblock)
	for range 3 {
		waitForWorkerSignal(t, started)
	}
	q.mu.Lock()
	claimed := len(q.claimed)
	q.mu.Unlock()
	if claimed != 3 || active.Load() != 3 {
		t.Fatalf("claimed = %d, active = %d, want exactly 3 of each", claimed, active.Load())
	}

	release <- struct{}{}
	waitForWorkerSignal(t, started)
	cancel()
	unblock()
	waitForWorkerSignal(t, done)

	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.claimed) != 4 || len(q.results) != 4 || len(q.pending) != 2 {
		t.Fatalf("claimed=%d, recorded=%d, pending=%d; want 4, 4, 2", len(q.claimed), len(q.results), len(q.pending))
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range q.claimed {
		if seen[id] {
			t.Errorf("delivery %s claimed twice", id)
		}
		seen[id] = true
	}
	for _, result := range q.results {
		if !result.Success || result.DisableThreshold != disableThreshold {
			t.Errorf("unexpected delivery result: %+v", result)
		}
	}
}

func TestDeliveryWorkersWakeFromIdle(t *testing.T) {
	q := newWorkerTestQueries(0)
	d := newWorkerTestDispatcher(t, q, func(*http.Request) (*http.Response, error) {
		return workerTestResponse(http.StatusOK), nil
	})
	_, done := startTestDeliveryWorkers(t, d)
	for range 3 {
		waitForWorkerSignal(t, q.idle)
	}
	q.mu.Lock()
	q.pending = append(q.pending, newWorkerTestQueries(1).pending...)
	q.mu.Unlock()
	d.wakeDeliveryWorkers()
	waitForWorkerSignal(t, q.recorded)
	select {
	case <-done:
		t.Fatal("workers stopped instead of waiting for more work")
	default:
	}
}

func TestDeliveryWorkerWakeupsAreCoalesced(t *testing.T) {
	d := newWorkerTestDispatcher(t, newWorkerTestQueries(0), nil)
	for range 100 {
		d.wakeDeliveryWorkers()
	}
	for i, wakeup := range d.wakeups {
		if len(wakeup) != 1 {
			t.Errorf("worker %d has %d wakeups, want 1", i, len(wakeup))
		}
	}
}

func TestDeliveryWorkerPreservesRetries(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		t.Run(fmt.Sprintf("exhausted=%v", exhausted), func(t *testing.T) {
			q := newWorkerTestQueries(1)
			if exhausted {
				q.pending[0].WebhookEventDelivery.RetryCount = int32(maxRetryCount)
			}
			d := newWorkerTestDispatcher(t, q, func(*http.Request) (*http.Response, error) {
				return workerTestResponse(http.StatusInternalServerError), nil
			})
			before := time.Now()
			d.drainPendingDeliveries(t.Context())
			after := time.Now()
			if len(q.results) != 1 || q.results[0].Success {
				t.Fatalf("expected one failed delivery result, got %+v", q.results)
			}
			if exhausted {
				if len(q.failed) != 1 || len(q.requeued) != 0 {
					t.Fatalf("failed=%d, requeued=%d; want 1, 0", len(q.failed), len(q.requeued))
				}
				return
			}
			if len(q.requeued) != 1 || len(q.failed) != 0 {
				t.Fatalf("requeued=%d, failed=%d; want 1, 0", len(q.requeued), len(q.failed))
			}
			retry := q.requeued[0]
			if retry.RetryCount != 1 || !retry.RunAt.Valid ||
				retry.RunAt.Time.Before(before.Add(retryBackoffs[0])) ||
				retry.RunAt.Time.After(after.Add(retryBackoffs[0])) {
				t.Errorf("unexpected retry: %+v", retry)
			}
		})
	}
}

type workerTestQueries struct {
	webhooksql.Querier
	mu       sync.Mutex
	pending  []*webhooksql.ClaimPendingDeliveriesRow
	claimed  []uuid.UUID
	results  []webhooksql.RecordDeliveryResultParams
	requeued []webhooksql.RequeueDeliveryParams
	failed   []uuid.UUID
	idle     chan struct{}
	recorded chan struct{}
}

func newWorkerTestQueries(count int) *workerTestQueries {
	q := &workerTestQueries{idle: make(chan struct{}, 100), recorded: make(chan struct{}, 100)}
	subID := uuid.New()
	for range count {
		q.pending = append(q.pending, &webhooksql.ClaimPendingDeliveriesRow{
			WebhookEventDelivery: webhooksql.WebhookEventDelivery{ID: uuid.New()},
			WebhookSubscription: webhooksql.WebhookSubscription{
				ID: subID, Url: "https://subscriber.example.test", Secret: "test-secret", Enabled: true,
			},
			ActivityLogEntry: webhooksql.ActivityLogEntry{
				ResourceType: "WORKER_TEST_RESOURCE", Action: "CREATED", ResourceName: "test-resource",
			},
		})
	}
	return q
}

func (q *workerTestQueries) ClaimPendingDeliveries(ctx context.Context, batchSize int32) ([]*webhooksql.ClaimPendingDeliveriesRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if batchSize != 1 {
		return nil, fmt.Errorf("claim size = %d, want 1", batchSize)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		q.idle <- struct{}{}
		return nil, nil
	}
	row := q.pending[0]
	q.pending = q.pending[1:]
	q.claimed = append(q.claimed, row.WebhookEventDelivery.ID)
	return []*webhooksql.ClaimPendingDeliveriesRow{row}, nil
}

func (q *workerTestQueries) CreateDelivery(ctx context.Context, arg webhooksql.CreateDeliveryParams) (*webhooksql.WebhookDelivery, error) {
	return &webhooksql.WebhookDelivery{
		ID: uuid.New(), Success: arg.Success, RequestBody: arg.RequestBody,
		ResponseStatus: arg.ResponseStatus,
		RequestHeaders: arg.RequestHeaders, ResponseHeaders: arg.ResponseHeaders,
		ResponseBody: arg.ResponseBody, ResponseBodyTruncated: arg.ResponseBodyTruncated,
		ErrorMessage: arg.ErrorMessage,
	}, ctx.Err()
}

func (q *workerTestQueries) MarkDeliveryCompleted(ctx context.Context, _ webhooksql.MarkDeliveryCompletedParams) error {
	return ctx.Err()
}

func (q *workerTestQueries) RecordDeliveryResult(ctx context.Context, arg webhooksql.RecordDeliveryResultParams) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.results = append(q.results, arg)
	q.recorded <- struct{}{}
	return false, nil
}

func (q *workerTestQueries) RequeueDelivery(_ context.Context, arg webhooksql.RequeueDeliveryParams) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.requeued = append(q.requeued, arg)
	return nil
}

func (q *workerTestQueries) MarkDeliveryFailed(_ context.Context, arg webhooksql.MarkDeliveryFailedParams) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.failed = append(q.failed, arg.ID)
	return nil
}

type workerRoundTripFunc func(*http.Request) (*http.Response, error)

func (f workerRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newWorkerTestDispatcher(t *testing.T, q *workerTestQueries, transport workerRoundTripFunc) *Dispatcher {
	t.Helper()
	log, _ := logrustest.NewNullLogger()
	d, err := NewDispatcher(nil, nil, "https://api.example.test", "test-tenant", false, log)
	if err != nil {
		t.Fatal(err)
	}
	d.queries = q
	d.httpClient.Transport = transport
	return d
}

func workerTestResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: http.NoBody, Header: make(http.Header)}
}

func startTestDeliveryWorkers(t *testing.T, d *Dispatcher) (context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	workers := d.startDeliveryWorkers(ctx)
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		waitForWorkerSignal(t, done)
	})
	return cancel, done
}

func waitForWorkerSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for delivery worker")
	}
}
