package sqlinstance

import (
	"context"
	"errors"
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	cpuUtilization    metricType = "cloudsql.googleapis.com/database/cpu/utilization"
	cpuCores          metricType = "cloudsql.googleapis.com/database/cpu/reserved_cores"
	memoryUtilization metricType = "cloudsql.googleapis.com/database/memory/utilization"
	memoryQuota       metricType = "cloudsql.googleapis.com/database/memory/quota"
	diskUtilization   metricType = "cloudsql.googleapis.com/database/disk/utilization"
	diskQuota         metricType = "cloudsql.googleapis.com/database/disk/quota"

	filter metricsFilter = `metric.type="%s"
		AND resource.type="cloudsql_database"`
)

type (
	metricsFilter = string
	metricType    = string
)

type metricsQuery struct {
	MetricType metricType
	Filter     metricsFilter
}

type Metrics struct {
	monitoring  *monitoring.MetricClient
	log         log.FieldLogger
	defaultOpts *MetricsOptions
	cache       *cache.Cache
}

type MetricsOptions struct {
	query       *metricsQuery
	aggregation *monitoringpb.Aggregation
	interval    *monitoringpb.TimeInterval
}

type Option func(*MetricsOptions)

type teamMetricsCache = map[metricType]map[string]float64

func NewMetrics(ctx context.Context, log log.FieldLogger, opts ...option.ClientOption) (*Metrics, error) {
	client, err := monitoring.NewMetricClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		monitoring: client,
		log:        log,
		defaultOpts: &MetricsOptions{
			interval: &monitoringpb.TimeInterval{
				StartTime: timestamppb.New(time.Now().Add(-1 * time.Hour)),
				EndTime:   timestamppb.New(time.Now()),
			},
		},
		cache: cache.New(30*time.Minute, 40*time.Minute),
	}, nil
}

func WithDefaultQuery(metricType metricType) Option {
	return func(o *MetricsOptions) {
		q := &metricsQuery{
			MetricType: metricType,
		}
		q.Filter = fmt.Sprintf(filter, metricType)
		o.query = q
	}
}

func (m *Metrics) Close() error {
	return m.monitoring.Close()
}

func (m *Metrics) averageForTeam(ctx context.Context, projectID string, metricType metricType) (map[string]float64, error) {
	entry, found := m.cache.Get(projectID)
	tc := teamMetricsCache{}
	if found {
		tc = entry.(teamMetricsCache)
		if idToMetricValues, found := tc[metricType]; found {
			m.log.Debugf("found metrics in cache for metricType %q", metricType)
			return idToMetricValues, nil
		}
	}

	ts, err := m.listTimeSeries(ctx, projectID, WithDefaultQuery(metricType))
	if err != nil {
		return nil, err
	}

	idToMetricValues := m.average(metricType, ts)
	tc[metricType] = idToMetricValues

	m.cache.Set(projectID, tc, cache.DefaultExpiration)

	return idToMetricValues, nil
}

func (m *Metrics) averageForDatabase(ctx context.Context, projectID string, metricType metricType, databaseID string) (float64, error) {
	averages, err := m.averageForTeam(ctx, projectID, metricType)
	if err != nil {
		return 0, err
	}

	teamMetrics := teamMetricsCache{}
	teamMetrics[metricType] = averages
	if dbMetric, found := metricFor(teamMetrics, metricType, databaseID); found {
		return dbMetric, nil
	}

	return 0, nil
}

func (m *Metrics) average(metricType metricType, ts []*monitoringpb.TimeSeries) map[string]float64 {
	averages := map[string]float64{}
	for _, t := range ts {
		sum := 0.0
		if t.Metric.Type != metricType {
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
		databaseID, ok := t.Resource.Labels["database_id"]
		if !ok {
			m.log.Error("database_id not found")
			continue
		}

		averages[databaseID] = sum / float64(len(t.Points))
	}
	return averages
}

func (m *Metrics) listTimeSeries(ctx context.Context, projectID string, opts ...Option) ([]*monitoringpb.TimeSeries, error) {
	options := m.defaultOpts
	for _, o := range opts {
		o(options)
	}

	if options.query == nil {
		return nil, fmt.Errorf("query is required")
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

	timeSeries := make([]*monitoringpb.TimeSeries, 0)
	for {
		met, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return timeSeries, nil
		} else if err != nil {
			m.log.WithError(err).Error("failed to get next time series")
			return nil, err
		}
		timeSeries = append(timeSeries, met)
	}
}

func (m *Metrics) cpuForSQLInstance(ctx context.Context, projectID, name string) (*SQLInstanceCPU, error) {
	databaseID := projectID + ":" + name
	cpu, err := m.averageForDatabase(ctx, projectID, cpuUtilization, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	cpuCores, err := m.averageForDatabase(ctx, projectID, cpuCores, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	return &SQLInstanceCPU{
		Utilization: cpu * 100,
		Cores:       cpuCores,
	}, nil
}

func (m *Metrics) memoryForSQLInstance(ctx context.Context, projectID, name string) (*SQLInstanceMemory, error) {
	databaseID := projectID + ":" + name
	memory, err := m.averageForDatabase(ctx, projectID, memoryUtilization, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	memoryQuota, err := m.averageForDatabase(ctx, projectID, memoryQuota, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	return &SQLInstanceMemory{
		Utilization: memory * 100,
		QuotaBytes:  int(memoryQuota),
	}, nil
}

func (m *Metrics) diskForSQLInstance(ctx context.Context, projectID, name string) (*SQLInstanceDisk, error) {
	databaseID := projectID + ":" + name
	disk, err := m.averageForDatabase(ctx, projectID, diskUtilization, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	diskQuota, err := m.averageForDatabase(ctx, projectID, diskQuota, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	return &SQLInstanceDisk{
		Utilization: disk * 100,
		QuotaBytes:  int(diskQuota),
	}, nil
}

func metricFor(teamMetrics teamMetricsCache, metricType metricType, databaseID string) (float64, bool) {
	idToMetricValues, found := teamMetrics[metricType]
	if !found {
		return 0, false
	}
	m, found := idToMetricValues[databaseID]
	if !found {
		return 0, false
	}
	return m, true
}
