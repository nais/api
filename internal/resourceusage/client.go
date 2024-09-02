package resourceusage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

const (
	appCPURequest      = `sum by (container) (kube_pod_container_resource_requests{namespace="%s", container="%s", resource="cpu",unit="core"})`
	appCPUUsage        = `sum by (container) (rate(container_cpu_usage_seconds_total{namespace="%s", container="%s"}[5m]))`
	appMemoryRequest   = `sum by (container) (kube_pod_container_resource_requests{namespace="%s", container="%s", resource="memory",unit="byte"})`
	appMemoryUsage     = `sum by (container) (container_memory_working_set_bytes{namespace="%s", container="%s"})`
	teamCPURequest     = `sum by (container, owner_kind) (kube_pod_container_resource_requests{namespace="%s", container!~%q, resource="cpu",unit="core"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamCPUUsage       = `sum by (container, owner_kind) (rate(container_cpu_usage_seconds_total{namespace="%s", container!~%q}[5m]) * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"} )`
	teamMemoryRequest  = `sum by (container, owner_kind) (kube_pod_container_resource_requests{namespace="%s", container!~%q, resource="memory",unit="byte"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamMemoryUsage    = `sum by (container, owner_kind) (container_memory_working_set_bytes{namespace="%s", container!~%q} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamsCPURequest    = `sum by (namespace, owner_kind) (kube_pod_container_resource_requests{namespace!~%q, container!~%q, resource="cpu",unit="core"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamsCPUUsage      = `sum by (namespace, owner_kind) (rate(container_cpu_usage_seconds_total{namespace!~%q, container!~%q}[5m]) * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamsMemoryRequest = `sum by (namespace, owner_kind) (kube_pod_container_resource_requests{namespace!~%q, container!~%q, resource="memory",unit="byte"} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
	teamsMemoryUsage   = `sum by (namespace, owner_kind) (container_memory_working_set_bytes{namespace!~%q, container!~%q} * on(pod,namespace) group_left(owner_kind) kube_pod_owner{owner_kind="ReplicaSet"})`
)

var (
	ignoredContainers = strings.Join([]string{"elector", "linkerd-proxy", "cloudsql-proxy", "secure-logs-fluentd", "secure-logs-configmap-reload", "secure-logs-fluentbit", "wonderwall"}, "|") + "||" // Adding "||" to the query filters data without container
	ignoredNamespaces = strings.Join([]string{"kube-system", "nais-system", "cnrm-system", "configconnector-operator-system", "linkerd", "gke-mcs", "gke-managed-system", "kyverno", "default", "kube-node-lease", "kube-public"}, "|")
)

type ResourceUsageClient interface {
	AppResourceRequest(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType) (float64, error)
	AppResourceUsage(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType) (float64, error)
	AppResourceUsageRange(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType, start time.Time, end time.Time, step int) ([]*model.UsageDataPoint, error)
	TeamUtilization(ctx context.Context, teamSlug slug.Slug, resourceType model.UsageResourceType) ([]*model.AppUtilizationData, error)
	TeamsUtilization(ctx context.Context, resourceType model.UsageResourceType) ([]*model.TeamUtilizationData, error)
}

type Client struct {
	prometheuses map[string]promv1.API
	log          logrus.FieldLogger
}

func New(clusters []string, tenant string, log logrus.FieldLogger) (ResourceUsageClient, error) {
	proms, err := promClients(clusters, tenant)
	if err != nil {
		return nil, err
	}

	return &Client{
		prometheuses: proms,
		log:          log,
	}, nil
}

func (c *Client) queryAll(ctx context.Context, query string) (map[string]prom.Vector, error) {
	ret := map[string]prom.Vector{}
	for env, prometheus := range c.prometheuses {
		v, err := c.query(ctx, query, prometheus)
		if err != nil {
			c.log.WithError(err).Errorf("failed to query prometheus in %s", env)
			continue
		}
		ret[env] = v
	}

	return ret, nil
}

func (c *Client) query(ctx context.Context, query string, prometheus promv1.API) (prom.Vector, error) {
	v, warnings, err := prometheus.Query(ctx, query, time.Now().Add(-5*time.Minute))
	if err != nil {
		return nil, err
	}

	if len(warnings) > 0 {
		return nil, fmt.Errorf("prometheus query warnings: %s", strings.Join(warnings, ", "))
	}

	vector, ok := v.(prom.Vector)
	if !ok {
		return nil, fmt.Errorf("expected prometheus vector, got %T", v)
	}

	return vector, nil
}

func (c *Client) TeamsUtilization(ctx context.Context, resourceType model.UsageResourceType) ([]*model.TeamUtilizationData, error) {
	reqQ := teamsMemoryRequest
	usageQ := teamsMemoryUsage

	if resourceType == model.UsageResourceTypeCPU {
		reqQ = teamsCPURequest
		usageQ = teamsCPUUsage
	}

	requested, err := c.queryAll(ctx, fmt.Sprintf(reqQ, ignoredNamespaces, ignoredContainers))
	if err != nil {
		return nil, err
	}

	ret := []*model.TeamUtilizationData{}

	for env, samples := range requested {
		for _, sample := range samples {
			ret = append(ret, &model.TeamUtilizationData{
				TeamSlug:    slug.Slug(sample.Metric["namespace"]),
				Requested:   float64(sample.Value),
				Environment: env,
			})
		}
	}

	used, err := c.queryAll(ctx, fmt.Sprintf(usageQ, ignoredNamespaces, ignoredContainers))
	if err != nil {
		return nil, err
	}

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

func (c *Client) TeamUtilization(ctx context.Context, teamSlug slug.Slug, resourceType model.UsageResourceType) ([]*model.AppUtilizationData, error) {
	reqQ := teamMemoryRequest
	usageQ := teamMemoryUsage

	if resourceType == model.UsageResourceTypeCPU {
		reqQ = teamCPURequest
		usageQ = teamCPUUsage
	}

	requested, err := c.queryAll(ctx, fmt.Sprintf(reqQ, teamSlug, ignoredContainers))
	if err != nil {
		return nil, err
	}

	ret := []*model.AppUtilizationData{}

	for env, samples := range requested {
		for _, sample := range samples {
			ret = append(ret, &model.AppUtilizationData{
				TeamSlug:  teamSlug,
				AppName:   string(sample.Metric["container"]),
				Env:       env,
				Requested: float64(sample.Value),
			})
		}
	}

	used, err := c.queryAll(ctx, fmt.Sprintf(usageQ, teamSlug, ignoredContainers))
	if err != nil {
		return nil, err
	}

	for _, samples := range used {
		for _, sample := range samples {
			for _, data := range ret {
				if data.AppName == string(sample.Metric["container"]) {
					data.Used = float64(sample.Value)
				}
			}
		}
	}

	return ret, nil
}

func (c *Client) AppResourceRequest(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType) (float64, error) {
	q := appMemoryRequest
	if resourceType == model.UsageResourceTypeCPU {
		q = appCPURequest
	}

	v, err := c.query(ctx, fmt.Sprintf(q, teamSlug, app), c.prometheuses[env])
	if err != nil {
		return 0, err
	}
	return ensuredVal(v), nil
}

func (c *Client) AppResourceUsage(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType) (float64, error) {
	q := appMemoryUsage
	if resourceType == model.UsageResourceTypeCPU {
		q = appCPUUsage
	}

	v, err := c.query(ctx, fmt.Sprintf(q, teamSlug, app), c.prometheuses[env])
	if err != nil {
		return 0, err
	}

	return ensuredVal(v), nil
}

func (c *Client) AppResourceUsageRange(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType, start time.Time, end time.Time, step int) ([]*model.UsageDataPoint, error) {
	q := appMemoryUsage
	if resourceType == model.UsageResourceTypeCPU {
		q = appCPUUsage
	}
	v, warnings, err := c.prometheuses[env].QueryRange(ctx, fmt.Sprintf(q, teamSlug, app), promv1.Range{Start: start, End: end, Step: time.Duration(step) * time.Second})
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

	ret := make([]*model.UsageDataPoint, 0)

	for _, sample := range matrix {
		for _, value := range sample.Values {
			ret = append(ret, &model.UsageDataPoint{
				Value:     float64(value.Value),
				Timestamp: value.Timestamp.Time(),
			})
		}
	}

	return ret, nil
}

func promClients(clusters []string, tenant string) (map[string]promv1.API, error) {
	ret := map[string]promv1.API{}

	for _, cluster := range clusters {
		client, err := api.NewClient(api.Config{Address: fmt.Sprintf("https://prometheus.%s.%s.cloud.nais.io", cluster, tenant)})
		if err != nil {
			return nil, err
		}

		ret[cluster] = promv1.NewAPI(client)
	}

	return ret, nil
}

func ensuredVal(v prom.Vector) float64 {
	if len(v) == 0 {
		return 0
	}

	return float64(v[0].Value)
}
