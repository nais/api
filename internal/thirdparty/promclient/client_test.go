package promclient

import (
	"context"
	"testing"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
)

// recordingAPI is a minimal promv1.API stub that records the query string
// passed to Query/QueryRange so tests can assert on it.
type recordingAPI struct {
	promv1.API
	gotQuery string
}

func (r *recordingAPI) Query(ctx context.Context, query string, ts time.Time, opts ...promv1.Option) (prom.Value, promv1.Warnings, error) {
	r.gotQuery = query
	return prom.Vector{}, nil, nil
}

func (r *recordingAPI) QueryRange(ctx context.Context, query string, rng promv1.Range, opts ...promv1.Option) (prom.Value, promv1.Warnings, error) {
	r.gotQuery = query
	return prom.Matrix{}, nil, nil
}

func TestRealClient_QueryRange_InjectsEnv(t *testing.T) {
	api := &recordingAPI{}
	c := &RealClient{mimirMetrics: api}

	query := `avg(disk_used_percent{service="opensearch-team-name"})`
	want := `avg(disk_used_percent{k8s_cluster_name="prod-gcp",service="opensearch-team-name"})`

	_, _, err := c.QueryRange(t.Context(), "prod-gcp", query, promv1.Range{})
	if err != nil {
		t.Fatalf("QueryRange() failed: %v", err)
	}

	if api.gotQuery != want {
		t.Errorf("QueryRange() did not inject environment matcher.\ngot:  %s\nwant: %s", api.gotQuery, want)
	}
}

func TestRealClient_QueryRange_NoEnv(t *testing.T) {
	api := &recordingAPI{}
	c := &RealClient{mimirMetrics: api}

	query := `avg(disk_used_percent{service="opensearch-team-name"})`

	_, _, err := c.QueryRange(t.Context(), "", query, promv1.Range{})
	if err != nil {
		t.Fatalf("QueryRange() failed: %v", err)
	}

	if api.gotQuery != query {
		t.Errorf("QueryRange() unexpectedly modified query.\ngot:  %s\nwant: %s", api.gotQuery, query)
	}
}
