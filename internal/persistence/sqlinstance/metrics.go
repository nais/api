package sqlinstance

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	cpuUtilization    metricType = "cloudsql.googleapis.com/database/cpu/utilization"
	cpuCores          metricType = "cloudsql.googleapis.com/database/cpu/reserved_cores"
	cpuUsage          metricType = "cloudsql.googleapis.com/database/cpu/usage_time"
	memoryUtilization metricType = "cloudsql.googleapis.com/database/memory/utilization"
	memoryQuota       metricType = "cloudsql.googleapis.com/database/memory/quota"
	memoryUsage       metricType = "cloudsql.googleapis.com/database/memory/total_usage" // Includes buffer/cache, skip `total_` to exclude buffer/cache
	diskUtilization   metricType = "cloudsql.googleapis.com/database/disk/utilization"
	diskQuota         metricType = "cloudsql.googleapis.com/database/disk/quota"
	diskUsage         metricType = "cloudsql.googleapis.com/database/disk/bytes_used"

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
	monitoring *monitoring.MetricClient
	log        log.FieldLogger
	cache      *cache.Cache
	sumCache   *cache.Cache
}

type MetricsOptions struct {
	query       *metricsQuery
	aggregation *monitoringpb.Aggregation
	interval    *monitoringpb.TimeInterval
}

type Option func(*MetricsOptions)

type teamSumMetricsCache struct {
	lock sync.RWMutex
	data map[metricType]float64
}

func (t *teamSumMetricsCache) Get(metricType metricType) (float64, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	v, ok := t.data[metricType]
	return v, ok
}

func (t *teamSumMetricsCache) Set(metricType metricType, data float64) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.data[metricType] = data
}

type teamMetricsCache struct {
	lock sync.RWMutex
	data map[metricType]map[string]float64
}

func (t *teamMetricsCache) Get(metricType metricType) (map[string]float64, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	v, ok := t.data[metricType]
	return v, ok
}

func (t *teamMetricsCache) Set(metricType metricType, data map[string]float64) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.data[metricType] = data
}

func NewMetrics(ctx context.Context, log log.FieldLogger, opts ...option.ClientOption) (*Metrics, error) {
	client, err := monitoring.NewMetricClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		monitoring: client,
		log:        log,
		cache:      cache.New(30*time.Minute, 40*time.Minute),
		sumCache:   cache.New(30*time.Minute, 40*time.Minute),
	}, nil
}

func withDefaultQuery(metricType metricType) Option {
	return func(o *MetricsOptions) {
		q := &metricsQuery{
			MetricType: metricType,
		}
		q.Filter = fmt.Sprintf(filter, metricType)
		o.query = q
	}
}

func withAggregation(a *monitoringpb.Aggregation) Option {
	return func(o *MetricsOptions) {
		o.aggregation = a
	}
}

func (m *Metrics) Close() error {
	return m.monitoring.Close()
}

func (m *Metrics) averageForTeam(ctx context.Context, projectID string, metricType metricType) (map[string]float64, error) {
	entry, found := m.cache.Get(projectID)
	tc := &teamMetricsCache{
		data: map[string]map[string]float64{},
	}
	if found {
		tc = entry.(*teamMetricsCache)
		if idToMetricValues, found := tc.Get(metricType); found {
			m.log.Debugf("found metrics in cache for metricType %q", metricType)
			return idToMetricValues, nil
		}
	}

	ts, err := m.listTimeSeries(ctx, projectID, withDefaultQuery(metricType))
	if err != nil {
		return nil, err
	}

	idToMetricValues := m.average(metricType, ts)
	tc.Set(metricType, idToMetricValues)

	m.cache.Set(projectID, tc, cache.DefaultExpiration)

	return idToMetricValues, nil
}

func (m *Metrics) averageForDatabase(ctx context.Context, projectID string, metricType metricType, databaseID string) (float64, error) {
	averages, err := m.averageForTeam(ctx, projectID, metricType)
	if err != nil {
		return 0, err
	}

	teamMetrics := &teamMetricsCache{
		data: map[string]map[string]float64{},
	}
	teamMetrics.Set(metricType, averages)
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

func (m *Metrics) sumForTeam(ctx context.Context, projectID string, metric metricType) (float64, error) {
	entry, found := m.sumCache.Get(projectID)
	tc := &teamSumMetricsCache{
		data: map[metricType]float64{},
	}
	if found {
		tc = entry.(*teamSumMetricsCache)
		if idToMetricValues, found := tc.Get(metric); found {
			m.log.Debugf("found metrics in cache for metricType %q", metric)
			return idToMetricValues, nil
		}
	}

	perSeriesAligner := monitoringpb.Aggregation_ALIGN_RATE
	switch metric {
	case memoryUsage, diskUsage, cpuCores, diskQuota, memoryQuota:
		perSeriesAligner = monitoringpb.Aggregation_ALIGN_MEAN
	}

	ts, err := m.listTimeSeries(ctx, projectID, withDefaultQuery(metric), withAggregation(&monitoringpb.Aggregation{
		CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_SUM,
		PerSeriesAligner:   perSeriesAligner,
		AlignmentPeriod:    &durationpb.Duration{Seconds: 60},
	}))
	if err != nil {
		fmt.Println(metric, "\n", err)
		return 0, err
	}

	sum := 0.0
	if len(ts) > 0 {
		t := ts[0]
		if len(t.Points) > 0 {
			sum = t.Points[len(t.Points)-1].Value.GetDoubleValue()
		}
	}
	tc.Set(metric, sum)

	m.sumCache.Set(projectID, tc, cache.DefaultExpiration)

	return sum, nil
}

func (m *Metrics) listTimeSeries(ctx context.Context, projectID string, opts ...Option) ([]*monitoringpb.TimeSeries, error) {
	options := &MetricsOptions{
		interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(time.Now().Add(-1 * time.Hour)),
			EndTime:   timestamppb.New(time.Now()),
		},
	}
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

func (m *Metrics) teamSummaryCPU(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesCPU, error) {
	usage, err := m.sumForTeam(ctx, projectID, cpuUsage)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	cores, err := m.sumForTeam(ctx, projectID, cpuCores)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	utilization := 0.0
	if cores > 0 {
		utilization = usage / cores
	}

	return &TeamServiceUtilizationSQLInstancesCPU{
		Used:        usage,
		Requested:   cores,
		Utilization: utilization,
	}, nil
}

func (m *Metrics) teamSummaryMemory(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesMemory, error) {
	usage, err := m.sumForTeam(ctx, projectID, memoryUsage)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	quota, err := m.sumForTeam(ctx, projectID, memoryQuota)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	utilization := 0.0
	if quota > 0 {
		utilization = usage / quota
	}

	return &TeamServiceUtilizationSQLInstancesMemory{
		Used:        int(usage),
		Requested:   int(quota),
		Utilization: utilization,
	}, nil
}

func (m *Metrics) teamSummaryDisk(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesDisk, error) {
	usage, err := m.sumForTeam(ctx, projectID, diskUsage)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	quota, err := m.sumForTeam(ctx, projectID, diskQuota)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	utilization := 0.0
	if quota > 0 {
		utilization = usage / quota
	}

	return &TeamServiceUtilizationSQLInstancesDisk{
		Used:        int(usage),
		Requested:   int(quota),
		Utilization: utilization,
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

func metricFor(teamMetrics *teamMetricsCache, metricType metricType, databaseID string) (float64, bool) {
	idToMetricValues, found := teamMetrics.Get(metricType)
	if !found {
		return 0, false
	}
	m, found := idToMetricValues[databaseID]
	if !found {
		return 0, false
	}
	return m, true
}
