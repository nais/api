package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/loki/v3/pkg/loghttp"
	"github.com/sirupsen/logrus"
)

type Client interface {
	// Tail returns a channel that will get log messages sent to it until the provided context is closed. The provided
	// filter is used to filter which log messages to receive.
	Tail(context.Context, *LogSubscriptionFilter) (<-chan *LogLine, error)

	// QueryRange returns at most limit log lines matching the LogQL log query within the interval [start, end), using
	// Loki's range query endpoint in forward direction. Loki returns the earliest matching log lines.
	QueryRange(ctx context.Context, query string, start, end time.Time, limit int) ([]*LogLine, error)

	// CountLogLines returns the number of log lines matching the LogQL log query within the interval [start, end).
	CountLogLines(ctx context.Context, query string, start, end time.Time) (int, error)
}

const (
	lokiRequestTimeout   = 30 * time.Second
	maxLokiResponseBytes = 64 << 20
	maxLokiErrorBytes    = 1 << 10
)

// LokiError is returned when Loki responds with a non-successful status code.
type LokiError struct {
	StatusCode int
	Message    string
}

func (e *LokiError) Error() string {
	return fmt.Sprintf("loki responded with status %d: %s", e.StatusCode, e.Message)
}

func DefaultLokiUrlGenerator(tenant string) (*url.URL, error) {
	u, err := url.Parse(fmt.Sprintf("wss://loki.%s.cloud.nais.io", tenant))
	if err != nil {
		return nil, fmt.Errorf("parse loki URL: %w", err)
	}
	return u, nil
}

type querier struct {
	loki       *url.URL
	httpClient *http.Client
	log        logrus.FieldLogger
}

type lokiUrlGeneratorFunc func(tenant string) (*url.URL, error)

type setup struct {
	urlGenerator lokiUrlGeneratorFunc
}

type OptionFunc func(*setup)

func WithLocalLoki(addr string) OptionFunc {
	u, err := url.Parse(addr)
	return func(s *setup) {
		s.urlGenerator = func(string) (*url.URL, error) {
			return u, err
		}
	}
}

func NewClient(tenant string, log logrus.FieldLogger, opts ...OptionFunc) (Client, error) {
	s := &setup{}
	for _, opt := range opts {
		opt(s)
	}

	if s.urlGenerator == nil {
		s.urlGenerator = DefaultLokiUrlGenerator
	}

	lokiUrl, err := s.urlGenerator(tenant)
	if err != nil {
		return nil, fmt.Errorf("unable to generate Loki URL for tenant %q: %v", tenant, err)
	}

	return &querier{
		loki:       lokiUrl,
		httpClient: &http.Client{},
		log:        log,
	}, nil
}

func (q *querier) Tail(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	query, err := filter.lokiQueryParameters()
	if err != nil {
		return nil, err
	}
	lokiURL := q.loki.JoinPath("loki", "api", "v1", "tail")
	lokiURL.RawQuery = query.Encode()

	conn, err := connect(ctx, lokiURL.String(), q.log)
	if err != nil {
		q.log.
			WithError(err).
			WithField("lokiUrl", lokiURL.String()).
			WithField("filter", filter).
			Error("unable to connect to Loki")
		return nil, err
	}

	logLines := make(chan *LogLine, 1)

	go func() {
		if err := streamLogLines(ctx, conn, logLines); err != nil {
			q.log.WithError(err).Errorf("streaming log lines")
			logLines <- &LogLine{
				Time:    time.Now(),
				Message: fmt.Sprintf("error streaming log lines: %v", err),
			}
		}
		close(logLines)
	}()

	return logLines, nil
}

func connect(ctx context.Context, urlStr string, log logrus.FieldLogger) (*websocket.Conn, error) {
	scopeHeader := http.Header{"X-Scope-OrgID": []string{"tenant"}}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, urlStr, scopeHeader)
	if err != nil {
		return nil, fmt.Errorf("connect to loki: %w", err)
	}

	go func() {
		<-ctx.Done()
		log.Debugf("closing log streamer connection")
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}()

	return conn, nil
}

func streamLogLines(ctx context.Context, conn *websocket.Conn, logLines chan *LogLine) error {
	for ctx.Err() == nil {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				return fmt.Errorf("read log message from loki: %w", err)
			}
			return nil
		}

		var resp loghttp.TailResponse
		if err := json.NewDecoder(bytes.NewReader(message)).Decode(&resp); err != nil {
			return fmt.Errorf("parse log message from loki: %w", err)
		}

		for _, stream := range resp.Streams {
			labels := make([]*LogLineLabel, 0)
			for k, v := range stream.Labels {
				labels = append(labels, &LogLineLabel{
					Key:   k,
					Value: v,
				})
			}

			for _, entry := range stream.Entries {
				logLines <- &LogLine{
					Time:    entry.Timestamp,
					Message: entry.Line,
					Labels:  labels,
				}
			}
		}
	}

	return nil
}

func (q *querier) QueryRange(ctx context.Context, query string, start, end time.Time, limit int) ([]*LogLine, error) {
	values := url.Values{}
	values.Set("query", query)
	values.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	values.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	values.Set("limit", strconv.Itoa(limit))
	values.Set("direction", "forward")

	resp, err := q.get(ctx, "query_range", values)
	if err != nil {
		return nil, err
	}

	if resp.Data.ResultType != loghttp.ResultTypeStream {
		return nil, fmt.Errorf("unexpected loki result type %q", resp.Data.ResultType)
	}

	streams, ok := resp.Data.Result.(loghttp.Streams)
	if !ok {
		return nil, fmt.Errorf("unexpected loki result %T", resp.Data.Result)
	}

	lines := make([]*LogLine, 0)
	for _, stream := range streams {
		labels := sortedLabels(stream.Labels)
		for _, entry := range stream.Entries {
			lines = append(lines, &LogLine{
				Time:    entry.Timestamp.UTC(),
				Message: entry.Line,
				Labels:  labels,
			})
		}
	}

	return lines, nil
}

func (q *querier) CountLogLines(ctx context.Context, query string, start, end time.Time) (int, error) {
	// Loki range vectors select the half-open window (t-r, t], and r only supports millisecond precision. We count the
	// millisecond aligned superset [end-r, end) by evaluating at end-1ns, and subtract the log lines in the remaining
	// sub-millisecond interval [end-r, start), which are fetched with nanosecond precision.
	length := end.Sub(start)
	r := ((length + time.Millisecond - 1) / time.Millisecond) * time.Millisecond

	// Dropping the error labels prevents Loki from failing the metric query on log lines with parser errors, without
	// changing which log lines are counted. Keeping a single label results in a single series.
	values := url.Values{}
	values.Set("query", fmt.Sprintf("sum(count_over_time(%s | drop __error__, __error_details__ | keep %s [%dms]))", query, clusterNameLabel, r.Milliseconds()))
	values.Set("time", strconv.FormatInt(end.Add(-time.Nanosecond).UnixNano(), 10))

	resp, err := q.get(ctx, "query", values)
	if err != nil {
		return 0, err
	}

	if resp.Data.ResultType != loghttp.ResultTypeVector {
		return 0, fmt.Errorf("unexpected loki result type %q", resp.Data.ResultType)
	}

	vector, ok := resp.Data.Result.(loghttp.Vector)
	if !ok {
		return 0, fmt.Errorf("unexpected loki result %T", resp.Data.Result)
	}

	var total int
	switch len(vector) {
	case 0:
	case 1:
		total = int(vector[0].Value)
	default:
		return 0, fmt.Errorf("unexpected number of series in loki count result: %d", len(vector))
	}

	if r > length {
		extra, err := q.QueryRange(ctx, query, end.Add(-r), start, maxLinesPerTimestamp)
		if err != nil {
			return 0, err
		}
		if len(extra) >= maxLinesPerTimestamp {
			return 0, ErrLokiCountNotExact
		}
		total -= len(extra)
	}

	return total, nil
}

// ErrLokiCountNotExact is returned when the number of log lines can not be computed exactly.
var ErrLokiCountNotExact = errors.New("loki count not exact")

func (q *querier) get(ctx context.Context, endpoint string, values url.Values) (*loghttp.QueryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, lokiRequestTimeout)
	defer cancel()

	u := q.httpURL().JoinPath("loki", "api", "v1", endpoint)
	u.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create loki request: %w", err)
	}
	req.Header.Set("X-Scope-OrgID", "tenant")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loki request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		q.log.WithField("status", resp.StatusCode).WithField("endpoint", endpoint).Warn("loki query failed")
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, maxLokiErrorBytes))
		return nil, &LokiError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(msg))}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLokiResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read loki response: %w", err)
	}
	if len(body) > maxLokiResponseBytes {
		return nil, ErrLokiResponseTooLarge
	}

	var ret loghttp.QueryResponse
	if err := json.Unmarshal(body, &ret); err != nil {
		return nil, fmt.Errorf("parse loki response: %w", err)
	}

	if ret.Status != "success" {
		return nil, fmt.Errorf("unexpected loki response status %q", ret.Status)
	}

	return &ret, nil
}

// ErrLokiResponseTooLarge is returned when the response from Loki exceeds maxLokiResponseBytes.
var ErrLokiResponseTooLarge = errors.New("loki response too large")

func (q *querier) httpURL() *url.URL {
	u := *q.loki
	switch u.Scheme {
	case "wss":
		u.Scheme = "https"
	case "ws":
		u.Scheme = "http"
	}
	return &u
}

func sortedLabels(set loghttp.LabelSet) []*LogLineLabel {
	labels := make([]*LogLineLabel, 0, len(set))
	for k, v := range set {
		labels = append(labels, &LogLineLabel{Key: k, Value: v})
	}
	slices.SortFunc(labels, func(a, b *LogLineLabel) int {
		return strings.Compare(a.Key, b.Key)
	})
	return labels
}
