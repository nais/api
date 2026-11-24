package usersyncer

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type syncMetrics struct {
	runs       metric.Int64Counter
	lastOK     metric.Int64Gauge
	duration   metric.Float64Histogram
	snapshot   metric.Int64Gauge
	changes    metric.Int64Counter
	exportRuns metric.Int64Counter
}

func newSyncMetrics(meter metric.Meter) (*syncMetrics, error) {
	runs, err := meter.Int64Counter("nais_usersync_runs", metric.WithDescription("Number of API user sync runs by result."))
	if err != nil {
		return nil, fmt.Errorf("create user sync runs counter: %w", err)
	}
	lastOK, err := meter.Int64Gauge("nais_usersync_last_success_timestamp_seconds", metric.WithDescription("Unix timestamp of the last successful API user sync."))
	if err != nil {
		return nil, fmt.Errorf("create user sync last success gauge: %w", err)
	}
	duration, err := meter.Float64Histogram("nais_usersync_duration_seconds",
		metric.WithDescription("Duration of API user sync in seconds, excluding the independent Zitadel export."),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 5, 10, 30, 60, 120, 240, 300, 600),
	)
	if err != nil {
		return nil, fmt.Errorf("create user sync duration histogram: %w", err)
	}
	snapshot, err := meter.Int64Gauge("nais_usersync_snapshot_users", metric.WithDescription("Eligible users in the latest complete source snapshots, after optional filtering."))
	if err != nil {
		return nil, fmt.Errorf("create user sync snapshot gauge: %w", err)
	}
	changes, err := meter.Int64Counter("nais_usersync_api_user_changes", metric.WithDescription("Number of committed API database user changes by action."))
	if err != nil {
		return nil, fmt.Errorf("create user sync changes counter: %w", err)
	}
	exportRuns, err := meter.Int64Counter("nais_usersync_zitadel_export_runs", metric.WithDescription("Number of tenant user exports to Zitadel by result."))
	if err != nil {
		return nil, fmt.Errorf("create Zitadel export runs counter: %w", err)
	}
	return &syncMetrics{runs: runs, lastOK: lastOK, duration: duration, snapshot: snapshot, changes: changes, exportRuns: exportRuns}, nil
}

func resultAttr(success bool) metric.MeasurementOption {
	if success {
		return metric.WithAttributes(attribute.String("result", "success"))
	}
	return metric.WithAttributes(attribute.String("result", "error"))
}

func (m *syncMetrics) recordRun(ctx context.Context, start, end time.Time, success bool) {
	m.runs.Add(ctx, 1, resultAttr(success))
	m.duration.Record(ctx, end.Sub(start).Seconds())
	if success {
		m.lastOK.Record(ctx, end.Unix())
	}
}

func (m *syncMetrics) recordSnapshot(ctx context.Context, source string, count int) {
	m.snapshot.Record(ctx, int64(count), metric.WithAttributes(attribute.String("source", source)))
}

func (m *syncMetrics) recordChanges(ctx context.Context, changes UserChanges) {
	m.changes.Add(ctx, int64(changes.Created), metric.WithAttributes(attribute.String("action", "create")))
	m.changes.Add(ctx, int64(changes.Updated), metric.WithAttributes(attribute.String("action", "update")))
	m.changes.Add(ctx, int64(changes.Deleted), metric.WithAttributes(attribute.String("action", "delete")))
}
