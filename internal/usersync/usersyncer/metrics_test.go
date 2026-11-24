package usersyncer

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

func TestSyncMetricsExport(t *testing.T) {
	ctx := t.Context()
	registry := prometheus.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry), otelprom.WithoutScopeInfo())
	if err != nil {
		t.Fatal(err)
	}
	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	m, err := newSyncMetrics(provider.Meter("usersync-test"))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Unix(100, 0)
	m.recordRun(ctx, start, start.Add(time.Second), true)
	m.recordRun(ctx, start.Add(10*time.Second), start.Add(12*time.Second), false)
	m.recordSnapshot(ctx, "google", 42)
	m.recordSnapshot(ctx, "nais_io", 0)
	m.recordChanges(ctx, UserChanges{Created: 3, Updated: 2, Deleted: 1})
	m.exportRuns.Add(ctx, 1, resultAttr(false))

	want := `
# HELP nais_usersync_api_user_changes_total Number of committed API database user changes by action.
# TYPE nais_usersync_api_user_changes_total counter
nais_usersync_api_user_changes_total{action="create"} 3
nais_usersync_api_user_changes_total{action="delete"} 1
nais_usersync_api_user_changes_total{action="update"} 2
# HELP nais_usersync_last_success_timestamp_seconds Unix timestamp of the last successful API user sync.
# TYPE nais_usersync_last_success_timestamp_seconds gauge
nais_usersync_last_success_timestamp_seconds 101
# HELP nais_usersync_runs_total Number of API user sync runs by result.
# TYPE nais_usersync_runs_total counter
nais_usersync_runs_total{result="error"} 1
nais_usersync_runs_total{result="success"} 1
# HELP nais_usersync_snapshot_users Eligible users in the latest complete source snapshots, after optional filtering.
# TYPE nais_usersync_snapshot_users gauge
nais_usersync_snapshot_users{source="google"} 42
nais_usersync_snapshot_users{source="nais_io"} 0
# HELP nais_usersync_zitadel_export_runs_total Number of tenant user exports to Zitadel by result.
# TYPE nais_usersync_zitadel_export_runs_total counter
nais_usersync_zitadel_export_runs_total{result="error"} 1
`
	if err := testutil.GatherAndCompare(registry, strings.NewReader(want),
		"nais_usersync_api_user_changes_total", "nais_usersync_last_success_timestamp_seconds", "nais_usersync_runs_total",
		"nais_usersync_snapshot_users", "nais_usersync_zitadel_export_runs_total"); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "nais_usersync_duration_seconds" {
			histogram := family.GetMetric()[0].GetHistogram()
			if histogram.GetSampleCount() != 2 || histogram.GetSampleSum() != 3 {
				t.Fatalf("expected two runs with total duration of 3 seconds, got %v", histogram)
			}
			return
		}
	}
	t.Fatal("user sync duration histogram not exported")
}
