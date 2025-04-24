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
)

const (
	appCPULimit         = `max by (container, namespace) (kube_pod_container_resource_limits{namespace=%q, container=%q, resource="cpu", unit="core"})`
	appCPURequest       = `max by (container, namespace) (kube_pod_container_resource_requests{namespace=%q, container=%q, resource="cpu",unit="core"})`
	appCPUUsage         = `rate(container_cpu_usage_seconds_total{namespace=%q, container=%q}[5m])`
	appMemoryLimit      = `max by (container, namespace) (kube_pod_container_resource_limits{namespace=%q, container=%q, resource="memory", unit="byte"})`
	appMemoryRequest    = `max by (container, namespace) (kube_pod_container_resource_requests{namespace=%q, container=%q, resource="memory",unit="byte"})`
	appMemoryUsage      = `last_over_time(container_memory_working_set_bytes{namespace=%q, container=%q}[5m])`
	instanceCPUUsage    = `rate(container_cpu_usage_seconds_total{namespace=%q, container=%q, pod=%q}[5m])`
	instanceMemoryUsage = `last_over_time(container_memory_working_set_bytes{namespace=%q, container=%q, pod=%q}[5m])`
	teamCPURequest      = `sum by (container, owner_kind) (kube_pod_container_resource_requests{namespace=%q, container!~%q, resource="cpu",unit="core"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamCPUUsage        = `sum by (container, owner_kind) (rate(container_cpu_usage_seconds_total{namespace=%q, container!~%q}[5m]) * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"} )`
	teamMemoryRequest   = `sum by (container, owner_kind) (kube_pod_container_resource_requests{namespace=%q, container!~%q, resource="memory",unit="byte"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamMemoryUsage     = `sum by (container, owner_kind) (container_memory_working_set_bytes{namespace=%q, container!~%q} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamsCPURequest     = `sum by (namespace, owner_kind) (kube_pod_container_resource_requests{namespace!~%q, container!~%q, resource="cpu",unit="core"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamsCPUUsage       = `sum by (namespace, owner_kind) (rate(container_cpu_usage_seconds_total{namespace!~%q, container!~%q}[5m]) * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamsMemoryRequest  = `sum by (namespace, owner_kind) (kube_pod_container_resource_requests{namespace!~%q, container!~%q, resource="memory",unit="byte"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamsMemoryUsage    = `sum by (namespace, owner_kind) (container_memory_working_set_bytes{namespace!~%q, container!~%q} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`

	cpuRequestRecommendation = `max(
		avg_over_time(
		  rate(container_cpu_usage_seconds_total{container=%q,namespace=%q}[5m])[1w:5m]
		)
	  and on ()
		(hour() >= %d and hour() < %d and day_of_week() > 0 and day_of_week() < 6)
	)`
	memoryRequestRecommendation = `max(
		avg_over_time(
		  quantile_over_time(0.8, container_memory_working_set_bytes{container=%q,namespace=%q}[5m])[1w:5m]
		)
	  and on ()
		time() >= (hour() >= %d and hour() < %d and day_of_week() > 0 and day_of_week() < 6)
	)`
	memoryLimitRecommendation = `max(
		max_over_time(
		  quantile_over_time(
			0.95,
			container_memory_working_set_bytes{container=%q,namespace=%q}[5m]
		  )[1w:5m]
		)
	  and on ()
		(hour() >= %d and hour() < %d and day_of_week() > 0 and day_of_week() < 6)
	)`

	minCPURequest         = 0.01             // 10m
	minMemoryRequestBytes = 32 * 1024 * 1024 // 64 MiB
)

var (
	ignoredContainers = strings.Join([]string{"elector", "linkerd-proxy", "cloudsql-proxy", "secure-logs-fluentd", "secure-logs-configmap-reload", "secure-logs-fluentbit", "wonderwall", "vks-sidecar"}, "|") + "||" // Adding "||" to the query filters data without container
	ignoredNamespaces = strings.Join([]string{"kube-system", "nais-system", "cnrm-system", "configconnector-operator-system", "linkerd", "gke-mcs", "gke-managed-system", "kyverno", "default", "kube-node-lease", "kube-public"}, "|")
)

func ForInstance(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, instanceName string, resourceType UtilizationResourceType) (*ApplicationInstanceUtilization, error) {
	usageQ := instanceMemoryUsage

	if resourceType == UtilizationResourceTypeCPU {
		usageQ = instanceCPUUsage
	}

	c := fromContext(ctx).client

	current, err := c.Query(ctx, env, fmt.Sprintf(usageQ, teamSlug, workloadName, instanceName))
	if err != nil {
		return nil, err
	}

	return &ApplicationInstanceUtilization{
		Current: ensuredVal(current),
	}, nil
}

func ForTeams(ctx context.Context, resourceType UtilizationResourceType) ([]*TeamUtilizationData, error) {
	reqQ := teamsMemoryRequest
	usageQ := teamsMemoryUsage

	if resourceType == UtilizationResourceTypeCPU {
		reqQ = teamsCPURequest
		usageQ = teamsCPUUsage
	}

	c := fromContext(ctx).client

	queryTime := time.Now().Add(-time.Minute * 5)
	requested, err := c.QueryAll(ctx, fmt.Sprintf(reqQ, ignoredNamespaces, ignoredContainers), promclient.WithTime(queryTime))
	if err != nil {
		return nil, err
	}

	ret := []*TeamUtilizationData{}

	for env, samples := range requested {
		for _, sample := range samples {
			ret = append(ret, &TeamUtilizationData{
				TeamSlug:        slug.Slug(sample.Metric["namespace"]),
				Requested:       float64(sample.Value),
				EnvironmentName: env,
			})
		}
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

	for env, samples := range used {
		for _, sample := range samples {
			for _, data := range ret {
				if data.TeamSlug == slug.Slug(sample.Metric["namespace"]) && data.EnvironmentName == env {
					data.Used = float64(sample.Value)
				}
			}
		}
	}

	return ret, nil
}

func ForTeam(ctx context.Context, teamSlug slug.Slug, resourceType UtilizationResourceType) ([]*WorkloadUtilizationData, error) {
	reqQ := teamMemoryRequest
	usageQ := teamMemoryUsage

	if resourceType == UtilizationResourceTypeCPU {
		reqQ = teamCPURequest
		usageQ = teamCPUUsage
	}

	c := fromContext(ctx).client

	requested, err := c.QueryAll(ctx, fmt.Sprintf(reqQ, teamSlug, ignoredContainers))
	if err != nil {
		return nil, err
	}

	ret := []*WorkloadUtilizationData{}

	for env, samples := range requested {
		for _, sample := range samples {
			ret = append(ret, &WorkloadUtilizationData{
				TeamSlug:        teamSlug,
				WorkloadName:    string(sample.Metric["container"]),
				EnvironmentName: env,
				Requested:       float64(sample.Value),
			})
		}
	}

	used, err := c.QueryAll(ctx, fmt.Sprintf(usageQ, teamSlug, ignoredContainers))
	if err != nil {
		return nil, err
	}

	for env, samples := range used {
		for _, sample := range samples {
			for _, data := range ret {
				if data.WorkloadName == string(sample.Metric["container"]) && data.EnvironmentName == env {
					data.Used = float64(sample.Value)
				}
			}
		}
	}

	return ret, nil
}

func WorkloadResourceRequest(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType) (float64, error) {
	q := appMemoryRequest
	if resourceType == UtilizationResourceTypeCPU {
		q = appCPURequest
	}

	c := fromContext(ctx).client

	v, err := c.Query(ctx, env, fmt.Sprintf(q, teamSlug, workloadName))
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

	v, err := c.Query(ctx, env, fmt.Sprintf(q, teamSlug, workloadName))
	if err != nil {
		return nil, err
	}

	if len(v) == 0 {
		return nil, nil
	}

	return (*float64)(&v[0].Value), nil
}

func WorkloadResourceUsage(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType) (float64, error) {
	q := appMemoryUsage
	if resourceType == UtilizationResourceTypeCPU {
		q = appCPUUsage
	}

	c := fromContext(ctx).client

	v, err := c.Query(ctx, env, fmt.Sprintf(q, teamSlug, workloadName))
	if err != nil {
		return 0, err
	}

	return ensuredVal(v), nil
}

func queryPrometheusRange(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, queryTemplate string, start time.Time, end time.Time, step int) ([]*UtilizationSample, error) {
	c := fromContext(ctx).client

	// Format the query
	query := fmt.Sprintf(queryTemplate, teamSlug, workloadName)

	// Perform the query
	v, warnings, err := c.QueryRange(ctx, env, query, promv1.Range{Start: start, End: end, Step: time.Duration(step) * time.Second})
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		return nil, fmt.Errorf("prometheus query warnings: %s", strings.Join(warnings, ", "))
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

	v, err := c.Query(ctx, env, fmt.Sprintf(cpuRequestRecommendation, workloadName, teamSlug, start.Hour(), start.Add(time.Hour*12).Hour()))
	if err != nil {
		return nil, err
	}

	cpuReq := ensuredVal(v)

	v, err = c.Query(ctx, env, fmt.Sprintf(memoryRequestRecommendation, workloadName, teamSlug, start.Hour(), start.Add(time.Hour*12).Hour()))
	if err != nil {
		return nil, err
	}

	memReq := ensuredVal(v)

	v, err = c.Query(ctx, env, fmt.Sprintf(memoryLimitRecommendation, workloadName, teamSlug, start.Hour(), start.Add(time.Hour*12).Hour()))
	if err != nil {
		return nil, err
	}

	memLimit := ensuredVal(v)

	return &WorkloadUtilizationRecommendations{
		CPURequestCores:    math.Max(cpuReq, minCPURequest),
		MemoryRequestBytes: int64(math.Max(roundUpToNearest32MiB(memReq), minMemoryRequestBytes)),
		MemoryLimitBytes:   int64(math.Max(roundUpToNearest32MiB(memLimit), minMemoryRequestBytes)),
	}, nil
}

func WorkloadResourceUsageRange(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType, start time.Time, end time.Time, step int) ([]*UtilizationSample, error) {
	queryTemplate := appMemoryUsage
	if resourceType == UtilizationResourceTypeCPU {
		queryTemplate = appCPUUsage
	}
	return queryPrometheusRange(ctx, env, teamSlug, workloadName, queryTemplate, start, end, step)
}

func WorkloadResourceRequestRange(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType, start time.Time, end time.Time, step int) ([]*UtilizationSample, error) {
	queryTemplate := appMemoryRequest
	if resourceType == UtilizationResourceTypeCPU {
		queryTemplate = appCPURequest
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
