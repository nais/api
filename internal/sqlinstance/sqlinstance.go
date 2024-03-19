package sqlinstance

import (
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"context"
	"fmt"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type SQLInstanceManager struct {
	monitoring *monitoring.MetricClient
}

const (
	CpuUtilizationFilter MetricsFilter = `metric.type = starts_with("cloudsql.googleapis.com/database/cpu/utilization")
		AND resource.type="cloudsql_database" 
		AND resource.labels.database_id = "%s"`
)

type MetricsFilter = string

type MetricsOptions struct {
	filter      MetricsFilter
	aggregation *monitoringpb.Aggregation
}

type Option func(*MetricsOptions)

func NewSQLInstanceManager(ctx context.Context) (*SQLInstanceManager, error) {
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}

	return &SQLInstanceManager{monitoring: client}, nil
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

func (m *SQLInstanceManager) Close() error {
	return m.monitoring.Close()
}

func (m *SQLInstanceManager) ListTimeSeries(ctx context.Context, projectID string, opts ...Option) ([]*monitoringpb.TimeSeries, error) {
	var options MetricsOptions
	for _, o := range opts {
		o(&options)
	}

	req := &monitoringpb.ListTimeSeriesRequest{
		Name: fmt.Sprintf("projects/%s", projectID),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   timestamppb.New(time.Now()),
			StartTime: timestamppb.New(time.Now().Add(-time.Hour * 24)),
		},
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
