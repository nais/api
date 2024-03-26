package sqlinstance

import (
	"context"
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Metrics struct {
	monitoring *monitoring.MetricClient
	log        log.FieldLogger
}

const (
	CpuUtilization    MetricType    = "cloudsql.googleapis.com/database/cpu/utilization"
	CpuCores          MetricType    = "cloudsql.googleapis.com/database/cpu/reserved_cores"
	MemoryUtilization MetricType    = "cloudsql.googleapis.com/database/memory/utilization"
	MemoryQuota       MetricType    = "cloudsql.googleapis.com/database/memory/quota"
	DiskUtilization   MetricType    = "cloudsql.googleapis.com/database/disk/utilization"
	DiskQuota         MetricType    = "cloudsql.googleapis.com/database/disk/quota"
	Filter            MetricsFilter = `metric.type = starts_with("%s")
		AND resource.type="cloudsql_database" 
		AND resource.labels.database_id = "%s"`
)

type (
	MetricsFilter = string
	MetricType    = string
)

type Query struct {
	MetricType MetricType
	Filter     MetricsFilter
}

type MetricsOptions struct {
	query       *Query
	aggregation *monitoringpb.Aggregation
	interval    *monitoringpb.TimeInterval
}

type Option func(*MetricsOptions)

func NewMetrics(ctx context.Context, log log.FieldLogger) (*Metrics, error) {
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		monitoring: client,
		log:        log,
	}, nil
}

func WithQuery(metricType MetricType, databaseId string) Option {
	return func(o *MetricsOptions) {
		q := &Query{
			MetricType: metricType,
		}
		q.Filter = fmt.Sprintf(Filter, metricType, databaseId)
		o.query = q
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

func (m *Metrics) AverageFor(ctx context.Context, projectID string, opts ...Option) (float64, error) {
	var options MetricsOptions
	for _, o := range opts {
		o(&options)
	}

	if options.query == nil {
		return 0, fmt.Errorf("query is required")
	}

	ts, err := m.ListTimeSeries(ctx, projectID, opts...)
	if err != nil {
		return 0, err
	}

	for _, t := range ts {
		sum := 0.0
		if t.Metric.Type != options.query.MetricType {
			continue
		}
		for _, p := range t.Points {
			switch t.ValueType {
			case metric.MetricDescriptor_INT64:
				sum += float64(p.Value.GetInt64Value())
			case metric.MetricDescriptor_DOUBLE:
				sum += p.Value.GetDoubleValue()
			default:
				// TODO: use injected logger
				log.Error("unsupported value type")
			}
		}
		return sum / float64(len(t.Points)), nil
	}
	return 0, nil
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

	if options.query != nil {
		req.Filter = options.query.Filter
	}

	it := m.monitoring.ListTimeSeries(ctx, req)

	timeSeries := make([]*monitoringpb.TimeSeries, 0)
	for {
		metric, err := it.Next()
		// TODO: handle error?
		if err != nil {
			m.log.WithError(err).Error("error when fetching time series")
			break
		}
		timeSeries = append(timeSeries, metric)
	}
	return timeSeries, nil
}
