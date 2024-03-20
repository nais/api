package sqlinstance

import (
	"context"
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Metrics struct {
	monitoring *monitoring.MetricClient
}

const (
	CpuUtilizationFilter MetricsFilter = `metric.type = starts_with("cloudsql.googleapis.com/database/cpu/utilization")
		AND resource.type="cloudsql_database" 
		AND resource.labels.database_id = "%s"`
	CpuUsageFilter MetricsFilter = `metric.type = starts_with("cloudsql.googleapis.com/database/cpu/usage_time")`
)

type MetricsFilter = string

type MetricsOptions struct {
	filter      MetricsFilter
	aggregation *monitoringpb.Aggregation
	interval    *monitoringpb.TimeInterval
}

type Option func(*MetricsOptions)

func NewMetrics(ctx context.Context) (*Metrics, error) {
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}

	return &Metrics{monitoring: client}, nil
}

func WithFilter(filter MetricsFilter, databaseId string) Option {
	return func(o *MetricsOptions) {
		o.filter = fmt.Sprintf(filter, databaseId)
	}
}

func WithAggregation(aggregation *monitoringpb.Aggregation) Option {
	return func(o *MetricsOptions) {
		o.aggregation = aggregation
	}
}

func WithInterval(start, end time.Time) Option {
	return func(o *MetricsOptions) {
		o.interval = &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(start),
			EndTime:   timestamppb.New(end),
		}
	}
}

func (m *Metrics) Close() error {
	return m.monitoring.Close()
}

func (m *Metrics) ListTimeSeries(ctx context.Context, projectID string, opts ...Option) ([]*monitoringpb.TimeSeries, error) {
	var options MetricsOptions
	for _, o := range opts {
		o(&options)
	}

	if options.interval == nil {
		options.interval = &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(time.Now().Add(-1 * time.Hour)),
			EndTime:   timestamppb.New(time.Now()),
		}
	}

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:        fmt.Sprintf("projects/%s", projectID),
		Interval:    options.interval,
		Aggregation: options.aggregation,
	}

	if options.filter != "" {
		req.Filter = options.filter
	}

	it := m.monitoring.ListTimeSeries(ctx, req)

	timeSeries := make([]*monitoringpb.TimeSeries, 0)
	for {
		metric, err := it.Next()
		// TODO: handle error?
		if err != nil {
			break
		}
		timeSeries = append(timeSeries, metric)
	}
	return timeSeries, nil
}
