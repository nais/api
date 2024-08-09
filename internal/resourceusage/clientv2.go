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
	memUtil = `
  (
      (
        sum by (namespace, container) (
			container_memory_working_set_bytes{container!~%[1]q,container=%[2]q,namespace=%[3]q}
        )
      )
    /
      (
        sum by (namespace, container) (
          kube_pod_container_resource_requests{container!~%[1]q,container=%[2]q,namespace=%[3]q,resource="memory",unit="byte"}
        )
      )
  )
*
  100
	`
	cpuUtil = `
  (
      (
        sum by (namespace, container) (
          rate(
            container_cpu_usage_seconds_total{container!~%[1]q,container=%[2]q,namespace=%[3]q}[5m]
          )
        )
      )
    /
      (
        sum by (namespace, container) (
          kube_pod_container_resource_requests{container!~%[1]q,container=%[2]q,namespace=%[3]q,resource="cpu",unit="core"}
        )
      )
  )
*
  100
	`
)

// var (
// 	namespacesToIgnore = []string{
// 		"default",
// 		"kube-system",
// 		"kyverno",
// 		"linkerd",
// 		"nais-system",
// 	}
//
// 	containersToIgnore = []string{
// 		"cloudsql-proxy",
// 		"elector",
// 		"linkerd-proxy",
// 		"secure-logs-configmap-reload",
// 		"secure-logs-fluentd",
// 		"vks-sidecar",
// 		"wonderwall",
// 	}
// )

type ClientV2 struct {
	prometheuses map[string]promv1.API
	log          logrus.FieldLogger
}

func NewClientV2(clusters []string, tenant string, log logrus.FieldLogger) (*ClientV2, error) {
	clients, err := getPrometheusClients(clusters, tenant)
	if err != nil {
		return nil, err
	}

	return &ClientV2{
		prometheuses: clients,
		log:          log,
	}, nil
}

func (c *ClientV2) ResourceRequestForApp(ctx context.Context, env string, team slug.Slug, app string, resourceType model.UtilizationResourceType) (float64, error) {
	return 699.69, nil
}

func (c *ClientV2) CurrentResourceUtilizationForApp(ctx context.Context, env string, team slug.Slug, app string, resourceType model.UtilizationResourceType) (float64, error) {
	return 69.69, nil
}

func (c *ClientV2) ResourceUtilizationForApp(ctx context.Context, env string, team slug.Slug, app string, resourceType model.UtilizationResourceType, start time.Time, end time.Time, step int) ([]*model.UtilizationDataPoint, error) {
	ignoredContainers := strings.Join(containersToIgnore, "|") + "|"
	q := memUtil
	if resourceType == model.UtilizationResourceTypeCPU {
		q = cpuUtil
	}
	v, warnings, err := c.prometheuses[env].QueryRange(ctx, fmt.Sprintf(q, ignoredContainers, app, team), promv1.Range{Start: start, End: end, Step: time.Duration(step) * time.Second})
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

	ret := make([]*model.UtilizationDataPoint, 0)

	for _, sample := range matrix {
		for _, value := range sample.Values {
			ret = append(ret, &model.UtilizationDataPoint{
				Value:     float64(value.Value),
				Timestamp: value.Timestamp.Time(),
			})
		}
	}

	return ret, nil
}

// getPrometheusClients will return a map of Prometheus clients, one for each cluster
// func getPrometheusClients(clusters []string, tenant string) (map[string]promv1.API, error) {
// 	clients := map[string]promv1.API{}
// 	for _, cluster := range clusters {
// 		client, err := api.NewClient(api.Config{
// 			Address: fmt.Sprintf("https://prometheus.%s.%s.cloud.nais.io", cluster, tenant),
// 		})
// 		if err != nil {
// 			return nil, err
// 		}
// 		clients[cluster] = promv1.NewAPI(client)
// 	}
// 	return clients, nil
// }

func getPrometheusClients(clusters []string, tenant string) (map[string]promv1.API, error) {
	client, _ := api.NewClient(api.Config{Address: "https://prometheus.dev-gcp.nav.cloud.nais.io"})
	return map[string]promv1.API{"dev": promv1.NewAPI(client)}, nil
}
