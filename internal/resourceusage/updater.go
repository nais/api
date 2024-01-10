package resourceusage

import (
	"context"
	"fmt"
	"github.com/nais/api/internal/database"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/k8s"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type (
	// utilizationMapForEnv is a map of team -> app -> time.Time -> *model.ResourceUtilization
	utilizationMapForEnv map[slug.Slug]map[string]map[time.Time]*model.ResourceUtilization
)

type Updater struct {
	k8sClient   *k8s.Client
	db          database.Database
	promClients map[string]promv1.API
	log         logrus.FieldLogger
}

const (
	cpuUsage      = `sum by (namespace, container) (rate(container_cpu_usage_seconds_total{namespace!~%q, container!~%q}[5m]))`
	cpuRequest    = `sum by (namespace, container) (kube_pod_container_resource_requests{namespace!~%q, container!~%q, resource="cpu", unit="core"})`
	memoryUsage   = `sum by (namespace, container) (container_memory_working_set_bytes{namespace!~%q, container!~%q})`
	memoryRequest = `sum by (namespace, container) (kube_pod_container_resource_requests{namespace!~%q, container!~%q, resource="memory", unit="byte"})`

	rangedQueryStep = time.Hour
)

var (
	namespacesToIgnore = []string{
		"default",
		"kube-system",
		"kyverno",
		"linkerd",
		"nais",
		"nais-system",
	}

	containersToIgnore = []string{
		"cloudsql-proxy",
		"elector",
		"linkerd-proxy",
		"secure-logs-configmap-reload",
		"secure-logs-fluentd",
		"vks-sidecar",
		"wonderwall",
	}
)

// NewUpdater creates a new resourceusage updater
func NewUpdater(k8sClient *k8s.Client, promClients map[string]promv1.API, db database.Database, log logrus.FieldLogger) *Updater {
	return &Updater{
		k8sClient:   k8sClient,
		db:          db,
		promClients: promClients,
		log:         log,
	}
}

// UpdateResourceUsage will update the resource usage from all added prometheus instances
func (u *Updater) UpdateResourceUsage(ctx context.Context) (rowsUpserted int, err error) {
	maxTimestamp, err := u.db.MaxResourceUtilizationDate(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to fetch max timestamp from database: %w", err)
	}

	start, end := getQueryRange(maxTimestamp.Time)

	resourceTypes := []gensql.ResourceType{
		gensql.ResourceTypeCpu,
		gensql.ResourceTypeMemory,
	}

	for env, promClient := range u.promClients {
		log := u.log.WithField("env", env)
		for _, resourceType := range resourceTypes {
			log = log.WithField("resource_type", resourceType)
			values, err := utilizationInEnv(ctx, env, u.k8sClient, promClient, resourceType, start, end, log)
			if err != nil {
				log.WithError(err).Errorf("unable to fetch resource usage")
				continue
			}

			batchErrors := 0
			batch := getBatchParams(resourceType, env, values)
			u.db.ResourceUtilizationUpsert(ctx, batch).Exec(func(i int, err error) {
				if err != nil {
					log.WithError(err).Errorf("unable to upsert resource utilization")
					batchErrors++
				}
			})
			log.WithFields(logrus.Fields{
				"num_rows":   len(batch),
				"num_errors": batchErrors,
			}).Debugf("batch upsert")
			rowsUpserted += len(batch) - batchErrors
		}
	}

	return rowsUpserted, nil
}

// utilizationInEnv returns resource utilization (usage and request) for all teams and apps in a given env
func utilizationInEnv(ctx context.Context, env string, k8sClient *k8s.Client, promClient promv1.API, resourceType gensql.ResourceType, start, end time.Time, log logrus.FieldLogger) (utilizationMapForEnv, error) {
	utilization := make(utilizationMapForEnv)
	usageQuery, requestQuery := getQueries(resourceType)

	log.WithField("query", usageQuery).Debugf("fetch usage data from prometheus")
	if usage, err := rangedQuery(ctx, promClient, usageQuery, start, end); err != nil {
		log.WithError(err).Errorf("unable to query prometheus for usage data")
	} else {
		for _, sample := range usage {
			team := string(sample.Metric["namespace"])
			app := string(sample.Metric["container"])
			log = log.WithFields(logrus.Fields{
				"team": team,
				"app":  app,
			})

			if !k8sClient.AppExists(env, team, app) {
				log.Debugf("app does not exist in kubernetes, skipping...")
				continue
			}

			for _, val := range sample.Values {
				ts := val.Timestamp.Time().UTC()
				usageValue := float64(val.Value)
				if usageValue == 0 {
					continue
				}

				if _, exists := utilization[slug.Slug(team)]; !exists {
					utilization[slug.Slug(team)] = make(map[string]map[time.Time]*model.ResourceUtilization)
				}

				if _, exists := utilization[slug.Slug(team)][app]; !exists {
					utilization[slug.Slug(team)][app] = make(map[time.Time]*model.ResourceUtilization)
				}

				utilization[slug.Slug(team)][app][ts] = &model.ResourceUtilization{
					Usage: usageValue,
				}
			}
		}
	}

	log.WithField("query", requestQuery).Debugf("fetch request data from prometheus")
	if request, err := rangedQuery(ctx, promClient, requestQuery, start, end); err != nil {
		log.WithError(err).Errorf("unable to query prometheus for request data")
	} else {
		for _, sample := range request {
			team := string(sample.Metric["namespace"])
			app := string(sample.Metric["container"])
			log = log.WithFields(logrus.Fields{
				"team": team,
				"app":  app,
			})

			if !k8sClient.AppExists(env, team, app) {
				log.Debugf("app does not exist in kubernetes, skipping...")
				continue
			}

			for _, val := range sample.Values {
				ts := val.Timestamp.Time().UTC()
				requestValue := float64(val.Value)
				if _, exists := utilization[slug.Slug(team)][app][ts]; !exists || requestValue == 0 {
					continue
				}

				utilization[slug.Slug(team)][app][ts].Request = requestValue
			}
		}
	}
	return utilization, nil
}

// getBatchParams converts ResourceUtilization to ResourceUtilizationUpsertParams
func getBatchParams(resourceType gensql.ResourceType, env string, utilization utilizationMapForEnv) []gensql.ResourceUtilizationUpsertParams {
	params := make([]gensql.ResourceUtilizationUpsertParams, 0)
	for team, apps := range utilization {
		for app, timestamps := range apps {
			for ts, value := range timestamps {
				if value.Usage == 0 || value.Request == 0 {
					continue
				}

				pgTs := &pgtype.Timestamptz{}
				_ = pgTs.Scan(ts)

				params = append(params, gensql.ResourceUtilizationUpsertParams{
					Timestamp:    *pgTs,
					Environment:  env,
					TeamSlug:     team,
					App:          app,
					ResourceType: resourceType,
					Usage:        value.Usage,
					Request:      value.Request,
				})
			}
		}
	}
	return params
}

// getQueries returns the prometheus queries for the given resource type
func getQueries(resourceType gensql.ResourceType) (usageQuery, requestQuery string) {
	if resourceType == gensql.ResourceTypeCpu {
		usageQuery = cpuUsage
		requestQuery = cpuRequest
	} else {
		usageQuery = memoryUsage
		requestQuery = memoryRequest
	}
	ignoreNamespaces := strings.Join(namespacesToIgnore, "|") + "|"
	ignoreContainers := strings.Join(containersToIgnore, "|") + "|"
	return fmt.Sprintf(usageQuery, ignoreNamespaces, ignoreContainers), fmt.Sprintf(requestQuery, ignoreNamespaces, ignoreContainers)
}

// getQueryRange returns the start and end time in UTC for a query, based on the given start time
func getQueryRange(start time.Time) (time.Time, time.Time) {
	now := time.Now()
	if start.IsZero() {
		start = now.AddDate(0, 0, -30)
	}

	end := start.Add(7 * 24 * time.Hour)
	if end.After(now) {
		end = now
	}

	return normalizeTime(start), normalizeTime(end)
}

// rangedQuery queries prometheus for the given query in the given time range
func rangedQuery(ctx context.Context, client promv1.API, query string, start, end time.Time) (prom.Matrix, error) {
	value, warnings, err := client.QueryRange(ctx, query, promv1.Range{
		Start: start,
		End:   end,
		Step:  rangedQueryStep,
	})
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		return nil, fmt.Errorf("prometheus query warnings: %s", strings.Join(warnings, ", "))
	}

	matrix, ok := value.(prom.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected prometheus matrix, got %T", value)
	}

	return matrix, nil
}

// normalizeTime will truncate a time.Time down to the hour, and return it as UTC
func normalizeTime(ts time.Time) time.Time {
	return ts.Truncate(time.Hour).UTC()
}
