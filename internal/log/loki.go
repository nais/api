package log

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/grafana/loki/v3/pkg/loghttp"
	"github.com/sirupsen/logrus"
)

type Querier interface {
	Query(context.Context, *LogSubscriptionFilter) ([]*LogLine, error)
}

type querier struct {
	logger  logrus.FieldLogger
	baseURL url.URL
	client  *http.Client
	orgID   string
}

func NewQuerier(ctx context.Context, lokiURL string, logger logrus.FieldLogger) (Querier, error) {
	client := &http.Client{}

	baseURLParsed, err := url.Parse(lokiURL)
	if err != nil {
		return nil, fmt.Errorf("parse loki URL: %w", err)
	}

	return &querier{
		client:  client,
		baseURL: *baseURLParsed,
		logger:  logger,
	}, nil
}

func (q *querier) Query(ctx context.Context, filter *LogSubscriptionFilter) ([]*LogLine, error) {
	if filter.opts.limit == 0 {
		filter.opts.limit = 100
	}
	if filter.opts.start.IsZero() {
		filter.opts.start = time.Now().Add(-10 * time.Hour)
	}
	if filter.opts.end.IsZero() {
		filter.opts.end = time.Now()
	}
	if filter.opts.direction == "" {
		filter.opts.direction = "BACKWARD"
	}
	params := url.Values{}
	params.Set("query", filter.Query())
	params.Set("limit", fmt.Sprintf("%d", filter.opts.limit))
	params.Set("start", fmt.Sprintf("%d", filter.opts.start.UnixNano()))
	params.Set("end", fmt.Sprintf("%d", filter.opts.end.UnixNano()))
	params.Set("direction", filter.opts.direction)

	queryURL := q.baseURL
	queryURL.Path = "/loki/api/v1/query_range"
	queryURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if q.orgID != "" {
		req.Header.Set("X-Scope-OrgID", q.orgID)
	}

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loki query failed with status %d", resp.StatusCode)
	}

	var queryResp loghttp.QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	lines := []*LogLine{}

	switch result := queryResp.Data.Result.(type) {
	case loghttp.Streams:
		for _, entries := range result {
			for _, entry := range entries.Entries {
				logLine := &LogLine{
					Time:    entry.Timestamp,
					Message: entry.Line,
					Labels:  entries.Labels,
				}
				lines = append(lines, logLine)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported result type: %s", result)
	}

	return lines, nil
}
