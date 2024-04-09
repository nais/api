package sqlinstance

import (
	"context"
	"errors"
	"fmt"
	"github.com/patrickmn/go-cache"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Metrics struct {
	monitoring *monitoring.MetricClient
	log        log.FieldLogger
	cache      *cache.Cache
}

const (
	CpuUtilization    MetricType = "cloudsql.googleapis.com/database/cpu/utilization"
	CpuCores          MetricType = "cloudsql.googleapis.com/database/cpu/reserved_cores"
	MemoryUtilization MetricType = "cloudsql.googleapis.com/database/memory/utilization"
	MemoryQuota       MetricType = "cloudsql.googleapis.com/database/memory/quota"
	DiskUtilization   MetricType = "cloudsql.googleapis.com/database/disk/utilization"
	DiskQuota         MetricType = "cloudsql.googleapis.com/database/disk/quota"

	Filter MetricsFilter = `metric.type = starts_with("%s")
		AND resource.type="cloudsql_database"`
	DatabaseIdFilter MetricsFilter = `metric.type = starts_with("%s")
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
		cache:      cache.New(5*time.Minute, 10*time.Minute),
	}, nil
}

func WithTeamQuery(metricType MetricType) Option {
	return func(o *MetricsOptions) {
		q := &Query{
			MetricType: metricType,
		}
		q.Filter = fmt.Sprintf(Filter, metricType)
		o.query = q
	}
}

func WithQuery(metricType MetricType, databaseId string) Option {
	return func(o *MetricsOptions) {
		q := &Query{
			MetricType: metricType,
		}
		q.Filter = fmt.Sprintf(DatabaseIdFilter, metricType, databaseId)
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

func (m *Metrics) AverageFor(ctx context.Context, projectID string, opts ...Option) (map[string]float64, error) {
	var options MetricsOptions
	for _, o := range opts {
		o(&options)
	}

	if options.query == nil {
		return nil, fmt.Errorf("query is required")
	}

	ts, err := m.ListTimeSeries(ctx, projectID, opts...)
	if err != nil {
		return nil, err
	}

	averages := make(map[string]float64)
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
				m.log.WithField("type", t.ValueType.String()).Error("unsupported value type")
			}
		}
		databaseId, ok := t.Resource.Labels["database_id"]
		if !ok {
			m.log.Error("database_id not found")
			continue
		}
		averages[databaseId] = sum / float64(len(t.Points))
	}
	return averages, nil
}

func (m *Metrics) ListTimeSeries(ctx context.Context, projectID string, opts ...Option) ([]*monitoringpb.TimeSeries, error) {
	var options MetricsOptions
	for _, o := range opts {
		o(&options)
	}

	cacheKey := projectID + ":" + options.query.Filter
	if _, ok := m.cache.Get(cacheKey); ok {
		m.log.Debug("cache hit")
		// return v.([]*monitoringpb.TimeSeries), nil
	}

	if options.interval == nil {
		options.interval = &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(time.Now().Add(-1 * time.Hour)),
			EndTime:   timestamppb.New(time.Now()),
		}
	}

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:        "projects/" + projectID,
		Interval:    options.interval,
		Aggregation: options.aggregation,
	}

	if options.query != nil {
		req.Filter = options.query.Filter
	}

	it := m.monitoring.ListTimeSeries(ctx, req)

	m.log.Debug("getting time series from monitoring api")
	timeSeries := make([]*monitoringpb.TimeSeries, 0)
	for {
		met, err := it.Next()
		if errors.Is(err, iterator.Done) {
			m.cache.Set(cacheKey, timeSeries, cache.DefaultExpiration)
			return timeSeries, nil
		} else if err != nil {
			m.log.WithError(err).Error("failed to get next time series")
			return nil, err
		}
		timeSeries = append(timeSeries, met)
	}
}
