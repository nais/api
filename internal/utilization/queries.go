package utilization

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
)

const (
	appCPULimit         = `kube_pod_container_resource_limits{namespace=%q, container=%q, resource="cpu", unit="core"}`
	appCPURequest       = `kube_pod_container_resource_requests{namespace=%q, container=%q, resource="cpu",unit="core"}`
	appCPUUsage         = `rate(container_cpu_usage_seconds_total{namespace=%q, container=%q}[5m])`
	appMemoryLimit      = `kube_pod_container_resource_limits{namespace=%q, container=%q, resource="memory", unit="byte"}`
	appMemoryRequest    = `kube_pod_container_resource_requests{namespace=%q, container=%q, resource="memory",unit="byte"}`
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

	current, err := c.query(ctx, env, fmt.Sprintf(usageQ, teamSlug, workloadName, instanceName))
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

	requested, err := c.queryAll(ctx, fmt.Sprintf(reqQ, ignoredNamespaces, ignoredContainers))
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

	used, err := c.queryAll(ctx, fmt.Sprintf(usageQ, ignoredNamespaces, ignoredContainers))
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

	for _, samples := range used {
		for _, sample := range samples {
			for _, data := range ret {
				if data.TeamSlug == slug.Slug(sample.Metric["namespace"]) {
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

	requested, err := c.queryAll(ctx, fmt.Sprintf(reqQ, teamSlug, ignoredContainers))
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

	used, err := c.queryAll(ctx, fmt.Sprintf(usageQ, teamSlug, ignoredContainers))
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

	v, err := c.query(ctx, env, fmt.Sprintf(q, teamSlug, workloadName))
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

	v, err := c.query(ctx, env, fmt.Sprintf(q, teamSlug, workloadName))
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

	v, err := c.query(ctx, env, fmt.Sprintf(q, teamSlug, workloadName))
	if err != nil {
		return 0, err
	}

	return ensuredVal(v), nil
}

func WorkloadResourceUsageRange(ctx context.Context, env string, teamSlug slug.Slug, workloadName string, resourceType UtilizationResourceType, start time.Time, end time.Time, step int) ([]*UtilizationSample, error) {
	q := appMemoryUsage
	if resourceType == UtilizationResourceTypeCPU {
		q = appCPUUsage
	}
	c := fromContext(ctx).client

	v, warnings, err := c.queryRange(ctx, env, fmt.Sprintf(q, teamSlug, workloadName), promv1.Range{Start: start, End: end, Step: time.Duration(step) * time.Second})
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		return nil, fmt.Errorf("prometheus query warnings: %s", strings.Join(warnings, ", "))
	}

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

func ensuredVal(v prom.Vector) float64 {
	if len(v) == 0 {
		return 0
	}

	return float64(v[0].Value)
}
