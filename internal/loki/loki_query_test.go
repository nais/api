package loki

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

// handlerTransport serves requests with an http.Handler without opening a network listener.
type handlerTransport struct {
	handler http.Handler
}

func (h handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.handler.ServeHTTP(rec, r)
	}()

	select {
	case <-done:
		return rec.Result(), nil
	case <-r.Context().Done():
		return nil, r.Context().Err()
	}
}

func newTestQuerier(t *testing.T, handler http.HandlerFunc) *querier {
	t.Helper()
	log, _ := test.NewNullLogger()
	c, err := NewClient("tenant", log, WithLocalLoki("http://loki.local:3100"))
	if err != nil {
		t.Fatal(err)
	}
	q := c.(*querier)
	q.httpClient = &http.Client{Transport: handlerTransport{handler: handler}}
	return q
}

func TestQuerier_QueryRange(t *testing.T) {
	var got url.Values
	q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/query_range" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("X-Scope-OrgID") != "tenant" {
			t.Errorf("missing tenant header")
		}
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[
			{"stream":{"service_name":"app","k8s_cluster_name":"dev"},"values":[["1792570000123456789","first"],["1792570000123456790","second"]]},
			{"stream":{"service_name":"other","k8s_cluster_name":"dev"},"values":[["1792570000123456789","third"]]}
		]}}`))
	})

	start := time.Unix(0, 1792570000000000001)
	end := time.Unix(0, 1792570001000000002)
	lines, err := q.QueryRange(context.Background(), `{service_name="app"}`, start, end, 101)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"query":     `{service_name="app"}`,
		"start":     "1792570000000000001",
		"end":       "1792570001000000002",
		"limit":     "101",
		"direction": "forward",
	}
	for k, v := range want {
		if got.Get(k) != v {
			t.Errorf("param %s: want %q, got %q", k, v, got.Get(k))
		}
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[1].Time.UnixNano() != 1792570000123456790 || lines[1].Message != "second" {
		t.Errorf("timestamp precision lost: %s", lines[1].Time.Format(time.RFC3339Nano))
	}
	if lines[0].Labels[0].Key != "k8s_cluster_name" || lines[0].Labels[1].Key != "service_name" {
		t.Errorf("expected sorted labels, got %v %v", lines[0].Labels[0], lines[0].Labels[1])
	}
}

func TestQuerier_CountLogLines(t *testing.T) {
	var got url.Values
	q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/query" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1792570001,"1234"]}]}}`))
	})

	start := time.Unix(0, 1792570000000000000)
	end := time.Unix(0, 1792570001000000000)
	n, err := q.CountLogLines(context.Background(), `{service_name="app", k8s_cluster_name="dev"} |= "x"`, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1234 {
		t.Errorf("expected 1234, got %d", n)
	}

	if got.Get("time") != "1792570000999999999" {
		t.Errorf("expected evaluation at end-1ns, got %s", got.Get("time"))
	}
	expr, err := syntax.ParseSampleExpr(got.Get("query"))
	if err != nil {
		t.Fatalf("count query is not valid LogQL: %v", err)
	}
	var ranges time.Duration
	expr.Walk(func(e syntax.Expr) bool {
		if r, ok := e.(*syntax.LogRangeExpr); ok {
			ranges = r.Interval
		}
		return true
	})
	if ranges != end.Sub(start) {
		t.Errorf("expected range %s, got %s for %s", end.Sub(start), ranges, expr)
	}
}

func TestQuerier_CountLogLinesSubMillisecond(t *testing.T) {
	start := time.Unix(0, 1792570000000000300)
	end := time.Unix(0, 1792570001000000500)

	var rangeQuery, countQuery url.Values
	q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loki/api/v1/query":
			countQuery = r.URL.Query()
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1792570001,"10"]}]}}`))
		case "/loki/api/v1/query_range":
			rangeQuery = r.URL.Query()
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[{"stream":{"a":"b"},"values":[["1792569999999000500","x"],["1792570000000000299","y"]]}]}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	n, err := q.CountLogLines(context.Background(), `{a="b"}`, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 {
		t.Errorf("expected 8, got %d", n)
	}

	expr, err := syntax.ParseSampleExpr(countQuery.Get("query"))
	if err != nil {
		t.Fatalf("count query is not valid LogQL: %v", err)
	}
	var interval time.Duration
	expr.Walk(func(e syntax.Expr) bool {
		if r, ok := e.(*syntax.LogRangeExpr); ok {
			interval = r.Interval
		}
		return true
	})
	if interval != time.Second+time.Millisecond {
		t.Errorf("expected range rounded up to 1.001s, got %s", interval)
	}
	if countQuery.Get("time") != "1792570001000000499" {
		t.Errorf("expected evaluation at end-1ns, got %s", countQuery.Get("time"))
	}
	if rangeQuery.Get("start") != "1792569999999000500" || rangeQuery.Get("end") != "1792570000000000300" {
		t.Errorf("unexpected correction interval %s - %s", rangeQuery.Get("start"), rangeQuery.Get("end"))
	}
}

func TestQuerier_CountLogLinesCorrectionTooLarge(t *testing.T) {
	q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loki/api/v1/query" {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
			return
		}
		var b strings.Builder
		b.WriteString(`{"status":"success","data":{"resultType":"streams","result":[{"stream":{"a":"b"},"values":[`)
		for i := range maxLinesPerTimestamp {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`["1792570000000000000","x"]`)
		}
		b.WriteString(`]}]}}`)
		_, _ = w.Write([]byte(b.String()))
	})
	_, err := q.CountLogLines(context.Background(), `{a="b"}`, time.Unix(0, 1792570000000000300), time.Unix(1, 0).Add(1792569999*time.Second))
	if !errors.Is(err, ErrLokiCountNotExact) {
		t.Fatalf("expected ErrLokiCountNotExact, got %v", err)
	}
}

func TestQuerier_CountLogLinesEmpty(t *testing.T) {
	q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	})
	n, err := q.CountLogLines(context.Background(), `{a="b"}`, time.Unix(0, 0), time.Unix(10, 0))
	if err != nil || n != 0 {
		t.Fatalf("expected 0, got %d %v", n, err)
	}
}

func TestQuerier_Errors(t *testing.T) {
	t.Run("status code", func(t *testing.T) {
		q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "max entries limit per query exceeded", http.StatusBadRequest)
		})
		_, err := q.QueryRange(context.Background(), `{a="b"}`, time.Unix(0, 0), time.Unix(1, 0), 10)
		var lokiErr *LokiError
		if !errors.As(err, &lokiErr) || lokiErr.StatusCode != http.StatusBadRequest || lokiErr.Message != "max entries limit per query exceeded" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unexpected result type", func(t *testing.T) {
		q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		})
		if _, err := q.QueryRange(context.Background(), `{a="b"}`, time.Unix(0, 0), time.Unix(1, 0), 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[{"stream":{},"values":[["x"`))
		})
		if _, err := q.QueryRange(context.Background(), `{a="b"}`, time.Unix(0, 0), time.Unix(1, 0), 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("canceled", func(t *testing.T) {
		started := make(chan struct{})
		q := newTestQuerier(t, func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
		})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-started
			cancel()
		}()

		done := make(chan error)
		go func() {
			_, err := q.QueryRange(ctx, `{a="b"}`, time.Unix(0, 0), time.Unix(1, 0), 10)
			done <- err
		}()

		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context.Canceled, got %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("request was not canceled")
		}
	})
}

func TestQuerier_HTTPURL(t *testing.T) {
	for in, want := range map[string]string{
		"wss://loki.tenant.cloud.nais.io": "https://loki.tenant.cloud.nais.io",
		"ws://localhost:3100":             "http://localhost:3100",
		"http://127.0.0.1:3100":           "http://127.0.0.1:3100",
	} {
		u, _ := url.Parse(in)
		q := &querier{loki: u, log: logrus.New()}
		if got := q.httpURL().String(); got != want {
			t.Errorf("%s: want %s, got %s", in, want, got)
		}
		if q.loki.String() != in {
			t.Errorf("original url modified: %s", q.loki)
		}
	}
}
