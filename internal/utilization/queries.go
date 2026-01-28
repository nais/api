package utilization

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/thirdparty/promclient"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

const (
	appCPULimit         = `max by (container, namespace) (kube_pod_container_resource_limits{k8s_cluster_name=%q, namespace=%q, container=%q, resource="cpu", unit="core"})`
	AppCPURequest       = `max by (container, namespace) (kube_pod_container_resource_requests{k8s_cluster_name=%q, namespace=%q, container=%q, resource="cpu",unit="core"})`
	AppCPUUsage         = `rate(container_cpu_usage_seconds_total{k8s_cluster_name=%q, namespace=%q, container=%q}[5m])`
	appCPUUsageAvg      = `avg by (container, namespace) (rate(container_cpu_usage_seconds_total{k8s_cluster_name=%q, namespace=%q, container=%q}[5m]))`
	appMemoryLimit      = `max by (container, namespace) (kube_pod_container_resource_limits{k8s_cluster_name=%q, namespace=%q, container=%q, resource="memory", unit="byte"})`
	AppMemoryRequest    = `max by (container, namespace) (kube_pod_container_resource_requests{k8s_cluster_name=%q, namespace=%q, container=%q, resource="memory",unit="byte"})`
	AppMemoryUsage      = `last_over_time(container_memory_working_set_bytes{k8s_cluster_name=%q, namespace=%q, container=%q}[5m])`
	appMemoryUsageAvg   = `avg by (container, namespace) (last_over_time(container_memory_working_set_bytes{k8s_cluster_name=%q, namespace=%q, container=%q}[5m]))`
	instanceCPUUsage    = `rate(container_cpu_usage_seconds_total{k8s_cluster_name=%q, namespace=%q, container=%q, pod=%q}[5m])`
	instanceMemoryUsage = `last_over_time(container_memory_working_set_bytes{k8s_cluster_name=%q, namespace=%q, container=%q, pod=%q}[5m])`

	TeamCPURequest    = `sum by (k8s_cluster_name, container, owner_kind) (kube_pod_container_resource_requests{namespace=%q, container!~%q, resource="cpu",unit="core"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	TeamCPUUsage      = `sum by (k8s_cluster_name, container, owner_kind) (rate(container_cpu_usage_seconds_total{namespace=%q, container!~%q}[5m]) * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"} )`
	TeamMemoryRequest = `sum by (k8s_cluster_name, container, owner_kind) (kube_pod_container_resource_requests{namespace=%q, container!~%q, resource="memory",unit="byte"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	TeamMemoryUsage   = `sum by (k8s_cluster_name, container, owner_kind) (container_memory_working_set_bytes{namespace=%q, container!~%q} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`

	TeamsCPURequest    = `sum by (k8s_cluster_name, namespace, owner_kind) (kube_pod_container_resource_requests{namespace!~%q, container!~%q, resource="cpu",unit="core"} * on(pod, namespace, k8s_cluster_name) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	TeamsCPUUsage      = `sum by (k8s_cluster_name, namespace, owner_kind) (rate(container_cpu_usage_seconds_total{namespace!~%q, container!~%q}[5m]) * on(pod, namespace, k8s_cluster_name) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	TeamsMemoryRequest = `sum by (k8s_cluster_name, namespace, owner_kind) (kube_pod_container_resource_requests{namespace!~%q, container!~%q, resource="memory",unit="byte"} * on(pod, namespace, k8s_cluster_name) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	TeamsMemoryUsage   = `sum by (k8s_cluster_name, namespace, owner_kind) (container_memory_working_set_bytes{namespace!~%q, container!~%q} * on(pod, namespace, k8s_cluster_name) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`

	cpuRequestRecommendation = `max(
		avg_over_time(
		  rate(container_cpu_usage_seconds_total{k8s_cluster_name=%q,namespace=%q, container=%q}[5m])[1w:5m]
		)
	  and on ()
		(hour() >= %d and hour() < %d and day_of_week() > 0 and day_of_week() < 6)
	)`
	memoryRequestRecommendation = `max(
		avg_over_time(
		  quantile_over_time(0.8, container_memory_working_set_bytes{k8s_cluster_name=%q,namespace=%q,container=%q}[5m])[1w:5m]
		)
	  and on ()
		time() >= (hour() >= %d and hour() < %d and day_of_week() > 0 and day_of_week() < 6)
	)`
	memoryLimitRecommendation = `max(
		max_over_time(
		  quantile_over_time(
			0.95,
			container_memory_working_set_bytes{k8s_cluster_name=%q,namespace=%q, container=%q}[5m]
		  )[1w:5m]
		)
	  and on ()
		(hour() >= %d and hour() < %d and day_of_week() > 0 and day_of_week() < 6)
	)`

	minCPURequest         = 0.01             // 10m
	minMemoryRequestBytes = 32 * 1024 * 1024 // 64 MiB
)

var (
	ignoredContainers = strings.Join([]string{"elector", "cloudsql-proxy", "secure-logs-fluentd", "secure-logs-configmap-reload", "secure-logs-fluentbit", "wonderwall", "vks-sidecar"}, "|") + "||" // Adding "||" to the query filters data without container
	ignoredNamespaces = strings.Join([]string{"kube-system", "nais-system", "cnrm-system", "configconnector-operator-system", "gke-mcs", "gke-managed-system", "kyverno", "default", "kube-node-lease", "kube-public"}, "|")
)

func ForInstance(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, instanceName string, resourceType UtilizationResourceType) (*ApplicationInstanceUtilization, error) {
	usageQ := instanceMemoryUsage

	if resourceType == UtilizationResourceTypeCPU {
		usageQ = instanceCPUUsage
	}

	c := fromContext(ctx).client

	current, err := c.Query(ctx, env, fmt.Sprintf(usageQ, env, teamSlug, workloadName, instanceName))
	if err != nil {
		return nil, err
	}

	return &ApplicationInstanceUtilization{
		Current: ensuredVal(current),
	}, nil
}

func ForTeams(ctx context.Context, resourceType UtilizationResourceType) ([]*TeamUtilizationData, error) {
	reqQ := TeamsMemoryRequest
	usageQ := TeamsMemoryUsage

	if resourceType == UtilizationResourceTypeCPU {
		reqQ = TeamsCPURequest
		usageQ = TeamsCPUUsage
	}

	c := fromContext(ctx).client

	queryTime := time.Now().Add(-time.Minute * 5)
	requested, err := c.QueryAll(ctx, fmt.Sprintf(reqQ, ignoredNamespaces, ignoredContainers), promclient.WithTime(queryTime))
	if err != nil {
		return nil, err
	}

	ret := []*TeamUtilizationData{}

	for _, sample := range requested {
		ret = append(ret, &TeamUtilizationData{
			TeamSlug:        slug.Slug(sample.Metric["namespace"]),
			Requested:       float64(sample.Value),
			EnvironmentName: string(sample.Metric["k8s_cluster_name"]),
		})
	}

	used, err := c.QueryAll(ctx, fmt.Sprintf(usageQ, ignoredNamespaces, ignoredContainers), promclient.WithTime(queryTime))
	if err != nil {
		return nil, err
	}

	slugs, err := team.ListAllSlugs(ctx)
	if err != nil {
		return nil, err
	}

	filtered := []*TeamUtilizationData{}
	for _, data := range ret {
		if slices.Contains(slugs, data.TeamSlug) {
			filtered = append(filtered, data)
		}
	}
	ret = filtered

	for _, sample := range used {
		for _, data := range ret {
			if data.TeamSlug == slug.Slug(sample.Metric["namespace"]) && data.EnvironmentName == string(sample.Metric["k8s_cluster_name"]) {
				data.Used = float64(sample.Value)
			}
		}
	}

	return ret, nil
}

func ForTeam(ctx context.Context, teamSlug slug.Slug, resourceType UtilizationResourceType) ([]*WorkloadUtilizationData, error) {
	reqQ := TeamMemoryRequest
	usageQ := TeamMemoryUsage

	if resourceType == UtilizationResourceTypeCPU {
		reqQ = TeamCPURequest
		usageQ = TeamCPUUsage
	}

	c := fromContext(ctx).client

	requested, err := c.QueryAll(ctx, fmt.Sprintf(reqQ, teamSlug, ignoredContainers))
	if err != nil {
		return nil, err
	}

	ret := []*WorkloadUtilizationData{}

	for _, sample := range requested {
		ret = append(ret, &WorkloadUtilizationData{
			TeamSlug:        teamSlug,
			WorkloadName:    string(sample.Metric["container"]),
			EnvironmentName: string(sample.Metric["k8s_cluster_name"]),
			Requested:       float64(sample.Value),
		})
	}

	used, err := c.QueryAll(ctx, fmt.Sprintf(usageQ, teamSlug, ignoredContainers))
	if err != nil {
		return nil, err
	}

	for _, sample := range used {
		for _, data := range ret {
			if data.WorkloadName == string(sample.Metric["container"]) && data.EnvironmentName == string(sample.Metric["k8s_cluster_name"]) {
				data.Used = float64(sample.Value)
			}
		}
	}

	return ret, nil
}

func WorkloadResourceRequest(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType) (float64, error) {
	q := AppMemoryRequest
	if resourceType == UtilizationResourceTypeCPU {
		q = AppCPURequest
	}

	c := fromContext(ctx).client

	v, err := c.Query(ctx, env, fmt.Sprintf(q, env, teamSlug, workloadName))
	if err != nil {
		return 0, err
	}
	return ensuredVal(v), nil
}

func WorkloadResourceLimit(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType) (*float64, error) {
	q := appMemoryLimit
	if resourceType == UtilizationResourceTypeCPU {
		q = appCPULimit
	}

	c := fromContext(ctx).client

	v, err := c.Query(ctx, env, fmt.Sprintf(q, env, teamSlug, workloadName))
	if err != nil {
		return nil, err
	}

	if len(v) == 0 {
		return nil, nil
	}

	return (*float64)(&v[0].Value), nil
}

func WorkloadResourceUsage(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType) (float64, error) {
	q := appMemoryUsageAvg
	if resourceType == UtilizationResourceTypeCPU {
		q = appCPUUsageAvg
	}

	c := fromContext(ctx).client

	v, err := c.Query(ctx, env, fmt.Sprintf(q, env, teamSlug, workloadName))
	if err != nil {
		return 0, err
	}

	return ensuredVal(v), nil
}

func queryPrometheusRange(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, queryTemplate string, start time.Time, end time.Time, step int) ([]*UtilizationSample, error) {
	c := fromContext(ctx).client

	// Format the query
	query := fmt.Sprintf(queryTemplate, env, teamSlug, workloadName)

	// Perform the query
	v, warnings, err := c.QueryRange(ctx, env, query, promv1.Range{Start: start, End: end, Step: time.Duration(step) * time.Second})
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		fromContext(ctx).log.WithFields(logrus.Fields{
			"environment": env,
			"warnings":    strings.Join(warnings, ", "),
		}).Warn("prometheus query warnings")
	}

	// Process the results
	matrix, ok := v.(prom.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected prometheus matrix, got %T", v)
	}

	ret := make([]*UtilizationSample, 0)
	for _, sample := range matrix {
		for _, value := range sample.Values {
			ret = append(ret, &UtilizationSample{
				Value:     float64(value.Value),
				Timestamp: value.Timestamp.Time(),
				Instance:  string(sample.Metric["pod"]),
			})
		}
	}

	// Sort the results by timestamp
	slices.SortStableFunc(ret, func(i, j *UtilizationSample) int {
		if i.Timestamp.Before(j.Timestamp) {
			return -1
		}
		if i.Timestamp.After(j.Timestamp) {
			return 1
		}
		return 0
	})

	return ret, nil
}

func WorkloadResourceRecommendations(ctx context.Context, env string, teamSlug slug.Slug, workloadName string) (*WorkloadUtilizationRecommendations, error) {
	c := fromContext(ctx).client
	now := time.Now().In(fromContext(ctx).location)

	start := time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, fromContext(ctx).location).UTC()

	return &WorkloadUtilizationRecommendations{
		client:          c,
		environmentName: env,
		workloadName:    workloadName,
		teamSlug:        teamSlug,
		start:           start,
	}, nil
}

func WorkloadResourceUsageRange(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType, start time.Time, end time.Time, step int) ([]*UtilizationSample, error) {
	queryTemplate := AppMemoryUsage
	if resourceType == UtilizationResourceTypeCPU {
		queryTemplate = AppCPUUsage
	}
	return queryPrometheusRange(ctx, env, teamSlug, workloadName, queryTemplate, start, end, step)
}

func WorkloadResourceRequestRange(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType, start time.Time, end time.Time, step int) ([]*UtilizationSample, error) {
	queryTemplate := AppMemoryRequest
	if resourceType == UtilizationResourceTypeCPU {
		queryTemplate = AppCPURequest
	}
	return queryPrometheusRange(ctx, env, teamSlug, workloadName, queryTemplate, start, end, step)
}

func WorkloadResourceLimitRange(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType, start time.Time, end time.Time, step int) ([]*UtilizationSample, error) {
	queryTemplate := appMemoryLimit
	if resourceType == UtilizationResourceTypeCPU {
		queryTemplate = appCPULimit
	}
	return queryPrometheusRange(ctx, env, teamSlug, workloadName, queryTemplate, start, end, step)
}

func ensuredVal(v prom.Vector) float64 {
	if len(v) == 0 {
		return 0
	}

	return float64(v[0].Value)
}

func roundUpToNearest32MiB(bytes float64) float64 {
	const chunk float64 = 32 * 1024 * 1024 // 32 MiB in bytes
	return math.Ceil(bytes/chunk) * chunk
}
