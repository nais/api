package loki

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/validate"
	"github.com/prometheus/prometheus/model/labels"
)

const (
	DefaultLogsPageSize = 100
	MaxLogsPageSize     = 1000

	// maxLinesPerTimestamp is the number of log lines requested when fetching all log lines sharing a single timestamp.
	// It must not exceed Loki's max_entries_limit_per_query (5000 by default). If a timestamp has this many log lines or
	// more, the result can not be paginated without risking loss, and an error is returned.
	maxLinesPerTimestamp = 5000
)

var minLogTime = time.Unix(0, 0).UTC()

type EnvironmentLogsFilter struct {
	Query string    `json:"query"`
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
}

type LogLineEdge = pagination.Edge[*LogLine]

type LogLineConnection struct {
	edges    []LogLineEdge
	pageInfo pagination.PageInfo
	count    func(ctx context.Context) (int, error)
}

func (c *LogLineConnection) Edges() []LogLineEdge {
	return c.edges
}

func (c *LogLineConnection) Nodes() []*LogLine {
	nodes := make([]*LogLine, len(c.edges))
	for i, edge := range c.edges {
		nodes[i] = edge.Node
	}
	return nodes
}

// PageInfo returns the page info for the connection. The total count requires an additional count query against Loki,
// and is only computed when the field is requested.
func (c *LogLineConnection) PageInfo(ctx context.Context) (*pagination.PageInfo, error) {
	pageInfo := c.pageInfo
	if graphql.GetFieldContext(ctx) == nil || !fieldRequested(ctx, "totalCount") {
		return &pageInfo, nil
	}

	total, err := c.count(ctx)
	if err != nil {
		return nil, err
	}
	pageInfo.TotalCount = total
	return &pageInfo, nil
}

func fieldRequested(ctx context.Context, name string) bool {
	for _, f := range graphql.CollectFieldsCtx(ctx, nil) {
		if f.Name == name {
			return true
		}
	}
	return false
}

// EnvironmentLogs returns log lines from Loki for the given environment within the half-open interval
// [filter.From, filter.To), ordered by timestamp ascending, then by labels and message.
func EnvironmentLogs(ctx context.Context, environmentName string, filter *EnvironmentLogsFilter, first *int, after *pagination.Cursor) (*LogLineConnection, error) {
	return environmentLogs(ctx, fromContext(ctx).client, environmentName, filter, first, after, time.Now())
}

func environmentLogs(ctx context.Context, client Client, environmentName string, filter *EnvironmentLogsFilter, first *int, after *pagination.Cursor, now time.Time) (*LogLineConnection, error) {
	query, limit, start, skip, anchor, occurrence, err := validateEnvironmentLogs(environmentName, filter, first, after, now)
	if err != nil {
		return nil, err
	}

	lines, hasNext, err := fetchLogPage(ctx, client, query, start, skip, anchor, occurrence, filter.To, limit)
	if err != nil {
		return nil, lokiQueryError(err)
	}

	pageInfo := pagination.PageInfo{
		HasNextPage:     hasNext,
		HasPreviousPage: !after.IsEmpty(),
	}
	if len(lines) > 0 {
		position, _ := after.Position()
		for i := range lines {
			lines[i].Cursor = lines[i].Cursor.WithPosition(position + i + 1)
		}
		pageInfo.StartCursor = &lines[0].Cursor
		pageInfo.EndCursor = &lines[len(lines)-1].Cursor
	}

	return &LogLineConnection{
		edges:    lines,
		pageInfo: pageInfo,
		count: func(ctx context.Context) (int, error) {
			total, err := client.CountLogLines(ctx, query, filter.From, filter.To)
			if err != nil {
				return 0, lokiQueryError(err)
			}
			return total, nil
		},
	}, nil
}

func validateEnvironmentLogs(environmentName string, filter *EnvironmentLogsFilter, first *int, after *pagination.Cursor, now time.Time) (query string, limit int, start time.Time, skip int, anchor string, occurrence int, err error) {
	verr := validate.New()

	query, qerr := restrictToEnvironment(filter.Query, environmentName)
	if qerr != nil {
		verr.Add("query", "%s", qerr.Error())
	}

	if filter.From.Before(minLogTime) {
		verr.Add("from", "The from time must not be before %s.", minLogTime.Format(time.RFC3339))
	}
	if filter.To.After(now) {
		verr.Add("to", "The to time must not be in the future.")
	}
	if !filter.From.Before(filter.To) {
		verr.Add("to", "The to time must be after the from time.")
	}

	limit = DefaultLogsPageSize
	if first != nil {
		limit = *first
		if limit < 1 || limit > MaxLogsPageSize {
			verr.Add("first", "The number of log lines must be between 1 and %d.", MaxLogsPageSize)
		}
	}

	start = filter.From
	if after != nil && !after.IsEmpty() {
		t, ok := after.Time()
		switch {
		case !ok:
			verr.Add("after", "Invalid cursor.")
		case t.Before(filter.From) || t.After(filter.To) || (t.Equal(filter.To) && after.Offset > 0):
			verr.Add("after", "The cursor is outside of the requested time interval.")
		case after.Offset < 0:
			verr.Add("after", "Invalid cursor.")
		default:
			start = t
			skip = after.Offset
			anchor, occurrence, _ = after.Anchor()
			if anchor != "" {
				skip = 0
			}
		}
	}

	if err := verr.NilIfEmpty(); err != nil {
		return "", 0, time.Time{}, 0, "", 0, err
	}

	return query, limit, start, skip, anchor, occurrence, nil
}

type queryValidationError string

func (e queryValidationError) Error() string { return string(e) }

// restrictToEnvironment parses a LogQL log query and restricts it to the cluster of the given environment. Metric
// queries are rejected, as are matchers on the cluster label selecting anything other than the given environment.
func restrictToEnvironment(query, environmentName string) (string, error) {
	expr, err := syntax.ParseLogSelector(query, false)
	if err != nil {
		return "", queryValidationError("Unable to parse log query. Only log queries are supported: " + err.Error())
	}

	cluster := environmentmapper.ClusterName(environmentName)
	var (
		selectors int
		walkErr   error
	)
	expr.Walk(func(e syntax.Expr) bool {
		m, ok := e.(*syntax.MatchersExpr)
		if !ok || walkErr != nil {
			return walkErr == nil
		}
		selectors++

		hasCluster := false
		for _, matcher := range m.Matchers() {
			if matcher.Name != clusterNameLabel {
				continue
			}
			if matcher.Type != labels.MatchEqual || matcher.Value != cluster {
				walkErr = queryValidationError("The query cannot select log lines from other environments. Remove the " + clusterNameLabel + " label from the query.")
				return false
			}
			hasCluster = true
		}

		if !hasCluster {
			m.AppendMatchers([]*labels.Matcher{labels.MustNewMatcher(labels.MatchEqual, clusterNameLabel, cluster)})
		}
		return true
	})
	if walkErr != nil {
		return "", walkErr
	}
	if selectors != 1 {
		return "", queryValidationError("The query must contain exactly one stream selector.")
	}

	restricted := expr.String()
	if _, err := syntax.ParseLogSelector(restricted, true); err != nil {
		return "", queryValidationError("Unable to parse log query: " + err.Error())
	}

	return restricted, nil
}

type lineWithKey struct {
	line      *LogLine
	labels    []*LogLineLabel
	anchorKey string
}

func sortLines(lines []*LogLine) []lineWithKey {
	ret := make([]lineWithKey, len(lines))
	for i, line := range lines {
		labels := slices.Clone(line.Labels)
		slices.SortFunc(labels, func(a, b *LogLineLabel) int {
			if c := strings.Compare(a.Key, b.Key); c != 0 {
				return c
			}
			return strings.Compare(a.Value, b.Value)
		})

		anchor := binary.AppendUvarint(nil, uint64(len(labels)))
		for _, label := range labels {
			anchor = binary.AppendUvarint(anchor, uint64(len(label.Key)))
			anchor = append(anchor, label.Key...)
			anchor = binary.AppendUvarint(anchor, uint64(len(label.Value)))
			anchor = append(anchor, label.Value...)
		}
		anchor = binary.AppendUvarint(anchor, uint64(len(line.Message)))
		anchor = append(anchor, line.Message...)
		ret[i] = lineWithKey{line: line, labels: labels, anchorKey: string(anchor)}
	}

	slices.SortStableFunc(ret, func(a, b lineWithKey) int {
		if c := a.line.Time.Compare(b.line.Time); c != 0 {
			return c
		}
		if c := compareLabels(a.labels, b.labels); c != 0 {
			return c
		}
		return strings.Compare(a.line.Message, b.line.Message)
	})

	return ret
}

func compareLabels(a, b []*LogLineLabel) int {
	for i := 0; i < min(len(a), len(b)); i++ {
		if c := strings.Compare(a[i].Key, b[i].Key); c != 0 {
			return c
		}
		if c := strings.Compare(a[i].Value, b[i].Value); c != 0 {
			return c
		}
	}
	return len(a) - len(b)
}

// appendGroups appends all lines, which must be sorted and contain complete timestamp groups, with cursors.
func appendGroups(out []LogLineEdge, lines []lineWithKey) []LogLineEdge {
	for i := 0; i < len(lines); {
		j := i
		for j < len(lines) && lines[j].line.Time.Equal(lines[i].line.Time) {
			j++
		}
		out = appendGroupSlice(out, lines[i:j], 0, j-i)
		i = j
	}
	return out
}

// appendGroupSlice appends group[from:from+n] where group contains all lines with the same timestamp.
func appendGroupSlice(out []LogLineEdge, group []lineWithKey, from, n int) []LogLineEdge {
	occurrences := make(map[string]int)
	for idx := range from + n {
		keyHash := sortKeyHash(group[idx].anchorKey)
		occurrences[keyHash]++
		if idx < from {
			continue
		}

		ts := group[idx].line.Time
		var cursor pagination.Cursor
		if idx+1 == len(group) {
			cursor = pagination.NewTimeCursor(ts.Add(time.Nanosecond), 0)
		} else {
			cursor = pagination.NewStableTimeCursor(ts, idx+1, group[idx].anchorKey, occurrences[keyHash])
		}
		out = append(out, LogLineEdge{Node: group[idx].line, Cursor: cursor})
	}
	return out
}

func fetchTimestampGroup(ctx context.Context, client Client, query string, ts time.Time) ([]lineWithKey, error) {
	lines, err := client.QueryRange(ctx, query, ts, ts.Add(time.Nanosecond), maxLinesPerTimestamp)
	if err != nil {
		return nil, err
	}
	if len(lines) >= maxLinesPerTimestamp {
		return nil, apierror.Errorf("There are %d or more log lines with the timestamp %s. Narrow the query to fetch these log lines.", maxLinesPerTimestamp, ts.Format(time.RFC3339Nano))
	}
	return sortLines(lines), nil
}

// fetchLogPage fetches at most limit log lines within [start, end), skipping the first skip log lines with timestamp
// exactly start. Pagination is lossless independent of how Loki orders log lines sharing a timestamp: a page only ends
// within a timestamp group when the complete group has been fetched and sorted deterministically.
func fetchLogPage(ctx context.Context, client Client, query string, start time.Time, skip int, anchor string, occurrence int, end time.Time, limit int) ([]LogLineEdge, bool, error) {
	out := make([]LogLineEdge, 0)

	if skip > 0 || anchor != "" {
		group, err := fetchTimestampGroup(ctx, client, query, start)
		if err != nil {
			return nil, false, err
		}
		if anchor != "" {
			skip = -1
			var seen int
			for i, line := range group {
				if sortKeyHash(line.anchorKey) == anchor {
					seen++
					if seen == occurrence {
						skip = i + 1
						break
					}
				}
			}
			if skip < 0 {
				return nil, false, apierror.Errorf("The log cursor no longer matches the result set. Restart pagination.")
			}
		}
		if skip < len(group) {
			if skip >= 0 {
				n := min(limit, len(group)-skip)
				out = appendGroupSlice(out, group, skip, n)
				if skip+n < len(group) {
					return out, true, nil
				}
			}
		}
		start = start.Add(time.Nanosecond)
	}

	if !start.Before(end) {
		return out, false, nil
	}

	remaining := limit - len(out)
	if remaining == 0 {
		probe, err := client.QueryRange(ctx, query, start, end, 1)
		if err != nil {
			return nil, false, err
		}
		return out, len(probe) > 0, nil
	}

	lines, err := client.QueryRange(ctx, query, start, end, remaining+1)
	if err != nil {
		return nil, false, err
	}
	sorted := sortLines(lines)

	if len(sorted) <= remaining {
		return appendGroups(out, sorted), false, nil
	}

	// Loki returned more lines than we can use. All lines before the last timestamp form complete groups, while the
	// group of the last timestamp may be incomplete.
	last := sorted[len(sorted)-1].line.Time
	complete := slices.IndexFunc(sorted, func(l lineWithKey) bool { return l.line.Time.Equal(last) })
	if complete > 0 {
		return appendGroups(out, sorted[:complete]), true, nil
	}

	group, err := fetchTimestampGroup(ctx, client, query, last)
	if err != nil {
		return nil, false, err
	}
	n := min(remaining, len(group))
	out = appendGroupSlice(out, group, 0, n)
	// When the group is exhausted we can not know whether more log lines exist, so we report a next page.
	return out, true, nil
}

func sortKeyHash(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func lokiQueryError(err error) error {
	var apiErr apierror.Error
	var lokiErr *LokiError
	switch {
	case errors.As(err, &apiErr):
		return apiErr
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return apierror.Errorf("The log query timed out. Narrow the query or the time interval and try again.")
	case errors.Is(err, ErrLokiCountNotExact):
		return apierror.Errorf("Unable to count the log lines exactly. Narrow the query or use a time interval with millisecond precision.")
	case errors.Is(err, ErrLokiResponseTooLarge):
		return apierror.Errorf("The log query returned too much data. Request fewer log lines and try again.")
	case errors.As(err, &lokiErr):
		switch {
		case lokiErr.StatusCode == http.StatusTooManyRequests:
			return apierror.Errorf("Loki is currently limiting requests. Please try again later.")
		case lokiErr.StatusCode >= 400 && lokiErr.StatusCode < 500:
			return apierror.Errorf("Loki rejected the log query: %s", lokiErr.Message)
		default:
			return apierror.Errorf("Loki failed to execute the log query (status %d). Please try again later.", lokiErr.StatusCode)
		}
	default:
		return apierror.Errorf("Unable to query Loki. Please try again later.")
	}
}
