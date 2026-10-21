package loki

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/validate"
	"github.com/vektah/gqlparser/v2/ast"
)

// fakeClient emulates Loki range queries: it returns the earliest `limit` lines within [start, end), but orders lines
// sharing a timestamp randomly to make sure pagination does not depend on Loki's tie ordering.
type fakeClient struct {
	lines   []*LogLine
	rnd     *rand.Rand
	err     error
	queries []string
	calls   int
	counts  int
}

func (f *fakeClient) Tail(context.Context, *LogSubscriptionFilter) (<-chan *LogLine, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeClient) QueryRange(ctx context.Context, query string, start, end time.Time, limit int) ([]*LogLine, error) {
	f.calls++
	f.queries = append(f.queries, query)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}

	matching := make([]*LogLine, 0)
	for _, l := range f.lines {
		if !l.Time.Before(start) && l.Time.Before(end) {
			matching = append(matching, l)
		}
	}
	f.rnd.Shuffle(len(matching), func(i, j int) { matching[i], matching[j] = matching[j], matching[i] })
	slices.SortStableFunc(matching, func(a, b *LogLine) int { return a.Time.Compare(b.Time) })
	if len(matching) > limit {
		matching = matching[:limit]
	}
	f.rnd.Shuffle(len(matching), func(i, j int) { matching[i], matching[j] = matching[j], matching[i] })
	return matching, nil
}

func (f *fakeClient) CountLogLines(_ context.Context, query string, start, end time.Time) (int, error) {
	f.counts++
	if f.err != nil {
		return 0, f.err
	}
	n := 0
	for _, l := range f.lines {
		if !l.Time.Before(start) && l.Time.Before(end) {
			n++
		}
	}
	return n, nil
}

var (
	testNow  = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	testFrom = time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	testTo   = time.Date(2026, 10, 21, 11, 0, 0, 0, time.UTC)
)

func line(ts time.Time, app, msg string) *LogLine {
	return &LogLine{
		Time:    ts,
		Message: msg,
		Labels: []*LogLineLabel{
			{Key: "k8s_cluster_name", Value: "dev"},
			{Key: "service_name", Value: app},
		},
	}
}

func filter() *EnvironmentLogsFilter {
	return &EnvironmentLogsFilter{Query: `{service_name="app"}`, From: testFrom, To: testTo}
}

func expectedOrder(lines []*LogLine, from, to time.Time) []string {
	sorted := sortLines(slices.DeleteFunc(slices.Clone(lines), func(l *LogLine) bool {
		return l.Time.Before(from) || !l.Time.Before(to)
	}))
	ret := make([]string, len(sorted))
	for i, l := range sorted {
		ret[i] = l.line.Message
	}
	return ret
}

func fetchAll(t *testing.T, client Client, f *EnvironmentLogsFilter, pageSize int) []string {
	t.Helper()
	var (
		after *pagination.Cursor
		got   []string
	)
	for page := 0; ; page++ {
		if page > 10_000 {
			t.Fatal("pagination does not terminate")
		}
		conn, err := environmentLogs(context.Background(), client, "dev", f, &pageSize, after, testNow)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if len(conn.edges) > pageSize {
			t.Fatalf("page %d: got %d lines, page size is %d", page, len(conn.edges), pageSize)
		}
		for _, e := range conn.edges {
			got = append(got, e.Node.Message)
		}
		if !conn.pageInfo.HasNextPage {
			return got
		}
		if conn.pageInfo.EndCursor == nil {
			t.Fatalf("page %d: has next page without end cursor", page)
		}
		after = roundTripCursor(t, *conn.pageInfo.EndCursor)
	}
}

func roundTripCursor(t *testing.T, c pagination.Cursor) *pagination.Cursor {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := c.MarshalGQLContext(context.Background(), buf); err != nil {
		t.Fatal(err)
	}
	ret := &pagination.Cursor{}
	if err := ret.UnmarshalGQLContext(context.Background(), strings.Trim(buf.String(), `"`)); err != nil {
		t.Fatal(err)
	}
	return ret
}

func TestEnvironmentLogs_PaginationIsLosslessWithSharedTimestamps(t *testing.T) {
	var lines []*LogLine
	n := 0
	add := func(ts time.Time, apps ...string) {
		for _, app := range apps {
			n++
			lines = append(lines, line(ts, app, fmt.Sprintf("line-%04d", n)))
		}
	}

	add(testFrom.Add(-time.Nanosecond), "app-a")
	add(testFrom, "app-a", "app-b")
	add(testFrom.Add(time.Nanosecond), "app-a")
	add(testFrom.Add(2*time.Nanosecond), "app-b")
	for i := range 40 {
		add(testFrom.Add(time.Second), fmt.Sprintf("app-%02d", i%7), "app-x")
	}
	for i := range 25 {
		add(testFrom.Add(time.Duration(i+2)*time.Second), "app-a")
	}
	add(testFrom.Add(time.Minute), "app-a", "app-a", "app-b", "app-c")
	add(testTo.Add(-time.Nanosecond), "app-z", "app-y")
	add(testTo, "app-a")

	want := expectedOrder(lines, testFrom, testTo)
	if len(want) != len(lines)-2 {
		t.Fatalf("test setup: expected all but two lines to be in range, got %d of %d", len(want), len(lines))
	}

	for _, pageSize := range []int{1, 2, 3, 5, 7, 39, 40, 41, 100, 1000} {
		for seed := range uint64(5) {
			t.Run(fmt.Sprintf("page size %d seed %d", pageSize, seed), func(t *testing.T) {
				client := &fakeClient{lines: lines, rnd: rand.New(rand.NewPCG(seed, seed))}
				got := fetchAll(t, client, filter(), pageSize)
				if !slices.Equal(want, got) {
					t.Errorf("unexpected lines\nwant: %v\ngot:  %v", want, got)
				}
			})
		}
	}
}

func TestEnvironmentLogs_SingleFullPage(t *testing.T) {
	lines := []*LogLine{
		line(testFrom.Add(3*time.Second), "app", "c"),
		line(testFrom.Add(time.Second), "app", "a"),
		line(testFrom.Add(2*time.Second), "app", "b"),
	}
	client := &fakeClient{lines: lines, rnd: rand.New(rand.NewPCG(1, 1))}

	conn, err := environmentLogs(context.Background(), client, "dev", filter(), new(3), nil, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if conn.pageInfo.HasNextPage || conn.pageInfo.HasPreviousPage {
		t.Errorf("expected no next or previous page, got %+v", conn.pageInfo)
	}
	if got := conn.Nodes(); len(got) != 3 || got[0].Message != "a" || got[2].Message != "c" {
		t.Errorf("unexpected nodes: %v", got)
	}
	if client.calls != 1 {
		t.Errorf("expected a single Loki query, got %d", client.calls)
	}
}

func TestEnvironmentLogs_PageInfoUsesAbsolutePositions(t *testing.T) {
	lines := []*LogLine{
		line(testFrom, "app", "a"),
		line(testFrom.Add(time.Second), "app", "b"),
		line(testFrom.Add(2*time.Second), "app", "c"),
		line(testFrom.Add(3*time.Second), "app", "d"),
	}
	client := &fakeClient{lines: lines, rnd: rand.New(rand.NewPCG(1, 1))}
	pageSize := 2

	first, err := environmentLogs(context.Background(), client, "dev", filter(), &pageSize, nil, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if first.pageInfo.PageStart() != 1 || first.pageInfo.PageEnd() != 2 {
		t.Fatalf("expected first page positions 1-2, got %d-%d", first.pageInfo.PageStart(), first.pageInfo.PageEnd())
	}

	after := roundTripCursor(t, *first.pageInfo.EndCursor)
	second, err := environmentLogs(context.Background(), client, "dev", filter(), &pageSize, after, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if second.pageInfo.PageStart() != 3 || second.pageInfo.PageEnd() != 4 {
		t.Fatalf("expected second page positions 3-4, got %d-%d", second.pageInfo.PageStart(), second.pageInfo.PageEnd())
	}
}

func TestEnvironmentLogs_ResumeAfterLateLineWithSameTimestamp(t *testing.T) {
	ts := testFrom.Add(time.Second)
	client := &fakeClient{
		lines: []*LogLine{
			line(ts, "app", "b"),
			line(ts, "app", "c"),
			line(ts, "app", "d"),
		},
		rnd: rand.New(rand.NewPCG(1, 1)),
	}
	pageSize := 2

	first, err := environmentLogs(context.Background(), client, "dev", filter(), &pageSize, nil, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{first.edges[0].Node.Message, first.edges[1].Node.Message}; !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("expected first page [b c], got %v", got)
	}

	client.lines = append(client.lines, line(ts, "app", "a"))
	after := roundTripCursor(t, *first.pageInfo.EndCursor)
	second, err := environmentLogs(context.Background(), client, "dev", filter(), &pageSize, after, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := second.Nodes(); len(got) != 1 || got[0].Message != "d" {
		messages := make([]string, len(got))
		for i, item := range got {
			messages[i] = item.Message
		}
		t.Fatalf("expected only d after inserting an earlier-sorting line, got %v", messages)
	}
}

func TestEnvironmentLogs_ResumeAfterIdenticalLines(t *testing.T) {
	ts := testFrom.Add(time.Second)
	client := &fakeClient{
		lines: []*LogLine{
			line(ts, "app", "same"),
			line(ts, "app", "same"),
			line(ts, "app", "same"),
			line(ts, "app", "last"),
		},
		rnd: rand.New(rand.NewPCG(1, 1)),
	}

	got := fetchAll(t, client, filter(), 1)
	want := []string{"last", "same", "same", "same"}
	if !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestEnvironmentLogs_EmptyResult(t *testing.T) {
	client := &fakeClient{rnd: rand.New(rand.NewPCG(1, 1))}
	conn, err := environmentLogs(context.Background(), client, "dev", filter(), nil, nil, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(conn.edges) != 0 || conn.pageInfo.HasNextPage || conn.pageInfo.StartCursor != nil || conn.pageInfo.EndCursor != nil {
		t.Errorf("unexpected empty result: %+v", conn.pageInfo)
	}
	if conn.Nodes() == nil {
		t.Error("expected empty, non-nil nodes")
	}
}

func TestEnvironmentLogs_NanosecondPrecision(t *testing.T) {
	base := time.Date(2026, 10, 21, 10, 30, 0, 123456789, time.UTC)
	lines := []*LogLine{
		line(base.Add(time.Nanosecond), "app", "second"),
		line(base, "app", "first"),
		line(base.Add(2*time.Nanosecond), "app", "third"),
	}
	client := &fakeClient{lines: lines, rnd: rand.New(rand.NewPCG(1, 1))}

	conn, err := environmentLogs(context.Background(), client, "dev", filter(), new(1), nil, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := conn.edges[0].Node.Time; !got.Equal(base) {
		t.Fatalf("expected %s, got %s", base.Format(time.RFC3339Nano), got.Format(time.RFC3339Nano))
	}

	cursor := roundTripCursor(t, *conn.pageInfo.EndCursor)
	ct, ok := cursor.Time()
	if !ok || !ct.Equal(base.Add(time.Nanosecond)) || cursor.Offset != 0 {
		t.Fatalf("unexpected cursor: %v %v %d", ct.Format(time.RFC3339Nano), ok, cursor.Offset)
	}

	conn, err = environmentLogs(context.Background(), client, "dev", filter(), new(1), cursor, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := conn.edges[0].Node; got.Message != "second" || !got.Time.Equal(base.Add(time.Nanosecond)) {
		t.Fatalf("unexpected line: %+v", got)
	}
}

func TestEnvironmentLogs_TooManyLinesForTimestamp(t *testing.T) {
	lines := make([]*LogLine, maxLinesPerTimestamp)
	for i := range lines {
		lines[i] = line(testFrom, "app", fmt.Sprintf("%d", i))
	}
	client := &fakeClient{lines: lines, rnd: rand.New(rand.NewPCG(1, 1))}

	conn, err := environmentLogs(context.Background(), client, "dev", filter(), new(10), nil, testNow)
	if conn != nil {
		t.Errorf("expected no result, got %d lines", len(conn.edges))
	}
	var apiErr apierror.Error
	if !errors.As(err, &apiErr) || !strings.Contains(err.Error(), "Narrow the query") {
		t.Fatalf("expected api error, got %v", err)
	}
}

func TestEnvironmentLogs_LokiErrors(t *testing.T) {
	lines := []*LogLine{line(testFrom, "app", "a")}
	tests := map[string]struct {
		err  error
		want string
	}{
		"bad request":       {err: &LokiError{StatusCode: http.StatusBadRequest, Message: "max entries limit"}, want: "Loki rejected the log query: max entries limit"},
		"rate limited":      {err: &LokiError{StatusCode: http.StatusTooManyRequests}, want: "limiting requests"},
		"server error":      {err: &LokiError{StatusCode: http.StatusBadGateway, Message: "secret internals"}, want: "status 502"},
		"timeout":           {err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded), want: "timed out"},
		"too large":         {err: ErrLokiResponseTooLarge, want: "too much data"},
		"connection failed": {err: errors.New("dial tcp: connection refused"), want: "Unable to query Loki"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client := &fakeClient{lines: lines, rnd: rand.New(rand.NewPCG(1, 1)), err: tc.err}
			conn, err := environmentLogs(context.Background(), client, "dev", filter(), nil, nil, testNow)
			if conn != nil {
				t.Error("expected no result on error")
			}
			var apiErr apierror.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected api error, got %T %v", err, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error containing %q, got %q", tc.want, err.Error())
			}
			if strings.Contains(err.Error(), "secret internals") {
				t.Errorf("server error details leaked: %q", err.Error())
			}
		})
	}
}

func TestEnvironmentLogs_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeClient{lines: []*LogLine{line(testFrom, "app", "a")}, rnd: rand.New(rand.NewPCG(1, 1))}

	conn, err := environmentLogs(ctx, client, "dev", filter(), nil, nil, testNow)
	if conn != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled and no result, got %v %v", conn, err)
	}
}

func TestEnvironmentLogs_Validation(t *testing.T) {
	offsetCursor := &pagination.Cursor{Offset: 3}
	outside := pagination.NewTimeCursor(testTo.Add(time.Second), 0)
	atToWithOffset := pagination.NewTimeCursor(testTo, 1)
	tests := map[string]struct {
		filter *EnvironmentLogsFilter
		first  *int
		after  *pagination.Cursor
		field  string
	}{
		"from equals to":           {filter: &EnvironmentLogsFilter{Query: `{a="b"}`, From: testFrom, To: testFrom}, field: "to"},
		"from after to":            {filter: &EnvironmentLogsFilter{Query: `{a="b"}`, From: testTo, To: testFrom}, field: "to"},
		"to in future":             {filter: &EnvironmentLogsFilter{Query: `{a="b"}`, From: testFrom, To: testNow.Add(time.Nanosecond)}, field: "to"},
		"zero from":                {filter: &EnvironmentLogsFilter{Query: `{a="b"}`, To: testTo}, field: "from"},
		"first zero":               {filter: filter(), first: new(0), field: "first"},
		"first too large":          {filter: filter(), first: new(MaxLogsPageSize + 1), field: "first"},
		"offset cursor":            {filter: filter(), after: offsetCursor, field: "after"},
		"cursor outside interval":  {filter: filter(), after: &outside, field: "after"},
		"cursor at to with offset": {filter: filter(), after: &atToWithOffset, field: "after"},
		"invalid query":            {filter: &EnvironmentLogsFilter{Query: `not logql`, From: testFrom, To: testTo}, field: "query"},
		"metric query":             {filter: &EnvironmentLogsFilter{Query: `count_over_time({a="b"}[1m])`, From: testFrom, To: testTo}, field: "query"},
		"other cluster":            {filter: &EnvironmentLogsFilter{Query: `{a="b", k8s_cluster_name="prod"}`, From: testFrom, To: testTo}, field: "query"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client := &fakeClient{rnd: rand.New(rand.NewPCG(1, 1))}
			_, err := environmentLogs(context.Background(), client, "dev", tc.filter, tc.first, tc.after, testNow)
			var verr *validate.ValidationErrors
			if !errors.As(err, &verr) {
				t.Fatalf("expected validation error, got %v", err)
			}
			found := false
			for _, e := range verr.Errors {
				if e.GraphQLField != nil && *e.GraphQLField == tc.field {
					found = true
				}
			}
			if !found {
				t.Errorf("expected error for field %q, got %+v", tc.field, verr.Errors)
			}
			if client.calls != 0 {
				t.Errorf("expected no Loki queries, got %d", client.calls)
			}
		})
	}
}

func TestEnvironmentLogs_TerminalCursorAtToIsAllowed(t *testing.T) {
	client := &fakeClient{lines: []*LogLine{line(testTo, "app", "outside")}, rnd: rand.New(rand.NewPCG(1, 1))}
	cursor := pagination.NewTimeCursor(testTo, 0)

	conn, err := environmentLogs(context.Background(), client, "dev", filter(), nil, &cursor, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(conn.edges) != 0 || conn.pageInfo.HasNextPage {
		t.Errorf("terminal cursor at to must not return out-of-range lines: %+v", conn.edges)
	}
}

func TestEnvironmentLogs_ValidBoundaries(t *testing.T) {
	client := &fakeClient{rnd: rand.New(rand.NewPCG(1, 1))}
	f := &EnvironmentLogsFilter{Query: `{a="b"}`, From: testFrom, To: testNow}
	if _, err := environmentLogs(context.Background(), client, "dev", f, new(MaxLogsPageSize), nil, testNow); err != nil {
		t.Fatalf("expected to equal now and max page size to be valid, got %v", err)
	}

	empty := &pagination.Cursor{}
	if err := empty.UnmarshalGQLContext(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	conn, err := environmentLogs(context.Background(), client, "dev", filter(), nil, empty, testNow)
	if err != nil {
		t.Fatalf("expected empty cursor to be accepted, got %v", err)
	}
	if conn.pageInfo.HasPreviousPage {
		t.Error("expected no previous page for empty cursor")
	}
}

func TestEnvironmentLogs_EnforcesEnvironmentInLokiQuery(t *testing.T) {
	environmentmapper.SetMapping(environmentmapper.EnvironmentMapping{"dev": "dev-gcp"})
	defer environmentmapper.SetMapping(nil)

	client := &fakeClient{rnd: rand.New(rand.NewPCG(1, 1))}
	f := &EnvironmentLogsFilter{Query: `{service_name="app"} |= "x"`, From: testFrom, To: testTo}
	if _, err := environmentLogs(context.Background(), client, "dev-gcp", f, nil, nil, testNow); err != nil {
		t.Fatal(err)
	}
	for _, q := range client.queries {
		if q != `{service_name="app", k8s_cluster_name="dev"} |= "x"` {
			t.Errorf("unexpected query sent to Loki: %s", q)
		}
	}
}

func TestRestrictToEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    string
		wantErr string
	}{
		{name: "injects cluster", query: `{service_name="app"}`, want: `{service_name="app", k8s_cluster_name="dev"}`},
		{name: "keeps pipeline", query: `{service_name="app"} | json | level="error"`, want: `{service_name="app", k8s_cluster_name="dev"} | json | level="error"`},
		{name: "same cluster is accepted", query: `{service_name="app", k8s_cluster_name="dev"}`, want: `{service_name="app", k8s_cluster_name="dev"}`},
		{name: "only cluster", query: `{k8s_cluster_name="dev"}`, want: `{k8s_cluster_name="dev"}`},
		{name: "label filter on cluster does not change selection", query: `{service_name="app"} | k8s_cluster_name="prod"`, want: `{service_name="app", k8s_cluster_name="dev"} | k8s_cluster_name="prod"`},
		{name: "label_format does not change selection", query: `{service_name="app"} | label_format k8s_cluster_name="prod"`, want: `{service_name="app", k8s_cluster_name="dev"} | label_format k8s_cluster_name="prod"`},
		{name: "other cluster", query: `{service_name="app", k8s_cluster_name="prod"}`, wantErr: "other environments"},
		{name: "regex cluster", query: `{service_name="app", k8s_cluster_name=~".+"}`, wantErr: "other environments"},
		{name: "regex matching own cluster", query: `{service_name="app", k8s_cluster_name=~"dev"}`, wantErr: "other environments"},
		{name: "negated cluster", query: `{service_name="app", k8s_cluster_name!="dev"}`, wantErr: "other environments"},
		{name: "own and other cluster", query: `{k8s_cluster_name="dev", k8s_cluster_name="prod"}`, wantErr: "other environments"},
		{name: "metric query", query: `sum(rate({service_name="app"}[5m]))`, wantErr: "Only log queries"},
		{name: "binary metric query", query: `count_over_time({service_name="app"}[1m]) / count_over_time({k8s_cluster_name="prod"}[1m])`, wantErr: "Only log queries"},
		{name: "invalid", query: `not logql`, wantErr: "Unable to parse"},
		{name: "empty", query: ``, wantErr: "Unable to parse"},
		{name: "empty compatible matcher is restricted", query: `{service_name=~".*"}`, want: `{service_name=~".*", k8s_cluster_name="dev"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := restrictToEnvironment(tc.query, "dev")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q, %v", tc.wantErr, got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("want %s, got %s", tc.want, got)
			}
		})
	}
}

func TestLogLineConnection_TotalCountOnlyWhenRequested(t *testing.T) {
	lines := []*LogLine{line(testFrom, "app", "a"), line(testFrom, "app", "b"), line(testTo, "app", "c")}
	client := &fakeClient{lines: lines, rnd: rand.New(rand.NewPCG(1, 1))}

	conn, err := environmentLogs(context.Background(), client, "dev", filter(), new(1), nil, testNow)
	if err != nil {
		t.Fatal(err)
	}

	pageInfoCtx := func(fields ...string) context.Context {
		sel := ast.SelectionSet{}
		for _, f := range fields {
			sel = append(sel, &ast.Field{Name: f, Alias: f})
		}
		ctx := graphql.WithOperationContext(context.Background(), &graphql.OperationContext{})
		return graphql.WithFieldContext(ctx, &graphql.FieldContext{Field: graphql.CollectedField{Field: &ast.Field{Name: "pageInfo", SelectionSet: sel}, Selections: sel}})
	}

	pi, err := conn.PageInfo(pageInfoCtx("hasNextPage", "endCursor"))
	if err != nil {
		t.Fatal(err)
	}
	if client.counts != 0 || !pi.HasNextPage {
		t.Fatalf("expected no count query and a next page, got %d counts, %+v", client.counts, pi)
	}

	pi, err = conn.PageInfo(pageInfoCtx("totalCount"))
	if err != nil {
		t.Fatal(err)
	}
	if client.counts != 1 || pi.TotalCount != 2 {
		t.Fatalf("expected total count 2 from one count query, got %d (%d queries)", pi.TotalCount, client.counts)
	}

	client.err = &LokiError{StatusCode: http.StatusInternalServerError}
	if _, err := conn.PageInfo(pageInfoCtx("totalCount")); err == nil {
		t.Fatal("expected count error to be returned")
	}
}
