package promclient

import (
	"context"
	"testing"
	"time"

	"github.com/nais/api/internal/slug"
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

func TestFilterRulesByTeam(t *testing.T) {
	teamSlug := slug.Slug("my-team")

	input := promv1.RulesResult{
		Groups: []promv1.RuleGroup{
			{
				Name: "group-fewer-than-3-parts-1",
				File: "cluster/my-team", // len == 2, should be skipped
				Rules: promv1.Rules{
					promv1.AlertingRule{Name: "Alert1"},
				},
			},
			{
				Name: "group-fewer-than-3-parts-2",
				File: "my-team", // len == 1, should be skipped
				Rules: promv1.Rules{
					promv1.AlertingRule{Name: "Alert1b"},
				},
			},
			{
				Name: "group-matching-team-and-env",
				File: "prod-gcp//my-team/alerts.yaml", // cluster=prod-gcp, namespace=my-team (3rd path element)
				Rules: promv1.Rules{
					promv1.AlertingRule{Name: "Alert2"},
					promv1.RecordingRule{Name: "Record1"}, // should be filtered out
				},
			},
			{
				Name: "group-matching-team-different-env",
				File: "dev-gcp//my-team/alerts.yaml", // cluster=dev-gcp, namespace=my-team (3rd path element)
				Rules: promv1.Rules{
					promv1.AlertingRule{Name: "Alert3"},
				},
			},
			{
				Name: "group-different-team",
				File: "prod-gcp//other-team/alerts.yaml", // namespace=other-team
				Rules: promv1.Rules{
					promv1.AlertingRule{Name: "Alert4"},
				},
			},
			{
				Name: "group-only-recording-rules",
				File: "prod-gcp//my-team/recording.yaml",
				Rules: promv1.Rules{
					promv1.RecordingRule{Name: "Record2"},
				},
			},
		},
	}

	t.Run("filter with specific environment", func(t *testing.T) {
		got := filterRulesByTeam(input, "prod-gcp", teamSlug)
		if len(got.Groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(got.Groups))
		}
		if got.Groups[0].Name != "group-matching-team-and-env" {
			t.Errorf("expected group 'group-matching-team-and-env', got %q", got.Groups[0].Name)
		}
		if len(got.Groups[0].Rules) != 1 {
			t.Fatalf("expected 1 rule in group, got %d", len(got.Groups[0].Rules))
		}
		alert, ok := got.Groups[0].Rules[0].(promv1.AlertingRule)
		if !ok || alert.Name != "Alert2" {
			t.Errorf("expected Alert2, got %#v", got.Groups[0].Rules[0])
		}
	})

	t.Run("filter with empty environment (matches all envs)", func(t *testing.T) {
		got := filterRulesByTeam(input, "", teamSlug)
		if len(got.Groups) != 2 {
			t.Fatalf("expected 2 groups, got %d", len(got.Groups))
		}
		expectedNames := map[string]bool{
			"group-matching-team-and-env":       true,
			"group-matching-team-different-env": true,
		}
		for _, g := range got.Groups {
			if !expectedNames[g.Name] {
				t.Errorf("unexpected group name: %s", g.Name)
			}
		}
	})
}
