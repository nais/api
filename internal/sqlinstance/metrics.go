package sqlinstance

import (
	"context"
	"errors"
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	CpuUtilization    MetricType = "cloudsql.googleapis.com/database/cpu/utilization"
	CpuCores          MetricType = "cloudsql.googleapis.com/database/cpu/reserved_cores"
	MemoryUtilization MetricType = "cloudsql.googleapis.com/database/memory/utilization"
	MemoryQuota       MetricType = "cloudsql.googleapis.com/database/memory/quota"
	DiskUtilization   MetricType = "cloudsql.googleapis.com/database/disk/utilization"
	DiskQuota         MetricType = "cloudsql.googleapis.com/database/disk/quota"

	Filter MetricsFilter = `metric.type="%s"
		AND resource.type="cloudsql_database"`
)

type (
	MetricsFilter = string
	MetricType    = string
)

type Query struct {
	MetricType MetricType
	Filter     MetricsFilter
}

type Metrics struct {
	monitoring  *monitoring.MetricClient
	log         log.FieldLogger
	defaultOpts *MetricsOptions
	cache       *cache.Cache
	costRepo    database.CostRepo
}

type MetricsOptions struct {
	query       *Query
	aggregation *monitoringpb.Aggregation
	interval    *monitoringpb.TimeInterval
}

type Option func(*MetricsOptions)

type DatabaseID string

type DatabaseIDToMetricValues = map[DatabaseID]float64

type TeamMetricsCache = map[MetricType]DatabaseIDToMetricValues

func NewMetrics(ctx context.Context, costRepo database.CostRepo, log log.FieldLogger) (*Metrics, error) {
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		costRepo:   costRepo,
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

func WithDefaultQuery(metricType MetricType) Option {
	return func(o *MetricsOptions) {
		q := &Query{
			MetricType: metricType,
		}
		q.Filter = fmt.Sprintf(Filter, metricType)
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

func (m *Metrics) AverageForTeam(ctx context.Context, projectID string, metricType MetricType) (map[DatabaseID]float64, error) {
	entry, found := m.cache.Get(projectID)
	tc := TeamMetricsCache{}
	if found {
		tc = entry.(TeamMetricsCache)
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

func (m *Metrics) averageForDatabase(ctx context.Context, projectID string, metricType MetricType, databaseID string) (float64, error) {
	averages, err := m.AverageForTeam(ctx, projectID, metricType)
	if err != nil {
		return 0, err
	}

	teamMetrics := TeamMetricsCache{}
	teamMetrics[metricType] = averages
	if dbMetric, found := metricFor(teamMetrics, metricType, DatabaseID(databaseID)); found {
		return dbMetric, nil
	}

	return 0, nil
}

func (m *Metrics) average(metricType MetricType, ts []*monitoringpb.TimeSeries) map[DatabaseID]float64 {
	averages := map[DatabaseID]float64{}
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
		databaseId, ok := t.Resource.Labels["database_id"]
		if !ok {
			m.log.Error("database_id not found")
			continue
		}

		averages[DatabaseID(databaseId)] = sum / float64(len(t.Points))
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

func (m *Metrics) metricsForSqlInstance(ctx context.Context, instance *model.SQLInstance) (*model.SQLInstanceMetrics, error) {
	databaseID := instance.ProjectID + ":" + instance.Name
	cpu, err := m.cpuForSqlInstance(ctx, instance.ProjectID, databaseID)
	if err != nil {
		return nil, err
	}

	disk, err := m.diskForSqlInstance(ctx, instance.ProjectID, databaseID)
	if err != nil {
		return nil, err
	}

	memory, err := m.memoryForSqlInstance(ctx, instance.ProjectID, databaseID)
	if err != nil {
		return nil, err
	}

	return &model.SQLInstanceMetrics{
		Cost:   m.costForSqlInstance(ctx, instance),
		CPU:    cpu,
		Disk:   disk,
		Memory: memory,
	}, nil
}

func (m *Metrics) costForSqlInstance(ctx context.Context, instance *model.SQLInstance) float64 {
	cost := 0.0
	if appName, exists := instance.GQLVars.Labels["app"]; exists {
		now := time.Now()
		var from, to pgtype.Date
		_ = to.Scan(now)
		_ = from.Scan(now.AddDate(0, 0, -30))

		if sum, err := m.costRepo.CostForInstance(ctx, "Cloud SQL", from, to, instance.GQLVars.TeamSlug, appName, instance.Env.Name); err != nil {
			m.log.WithError(err).Errorf("fetching cost")
		} else {
			cost = float64(sum)
		}
	}
	return cost
}

func (m *Metrics) cpuForSqlInstance(ctx context.Context, projectID, databaseID string) (*model.SQLInstanceCPU, error) {
	cpu, err := m.averageForDatabase(ctx, projectID, CpuUtilization, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	cpuCores, err := m.averageForDatabase(ctx, projectID, CpuCores, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	return &model.SQLInstanceCPU{
		Utilization: cpu * 100,
		Cores:       cpuCores,
	}, nil
}

func (m *Metrics) memoryForSqlInstance(ctx context.Context, projectID, databaseID string) (*model.SQLInstanceMemory, error) {
	memory, err := m.averageForDatabase(ctx, projectID, MemoryUtilization, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	memoryQuota, err := m.averageForDatabase(ctx, projectID, MemoryQuota, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	return &model.SQLInstanceMemory{
		Utilization: memory * 100,
		QuotaBytes:  int(memoryQuota),
	}, nil
}

func (m *Metrics) diskForSqlInstance(ctx context.Context, projectID, databaseID string) (*model.SQLInstanceDisk, error) {
	disk, err := m.averageForDatabase(ctx, projectID, DiskUtilization, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	diskQuota, err := m.averageForDatabase(ctx, projectID, DiskQuota, databaseID)
	if err != nil {
		return nil, apierror.ErrGoogleCloudMonitoringMetricsApi
	}

	return &model.SQLInstanceDisk{
		Utilization: disk * 100,
		QuotaBytes:  int(diskQuota),
	}, nil
}

func metricFor(teamMetrics TeamMetricsCache, metricType MetricType, databaseID DatabaseID) (float64, bool) {
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
