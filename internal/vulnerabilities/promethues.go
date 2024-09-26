package vulnerabilities

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/graph/model"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	prom_model "github.com/prometheus/common/model"
)

const (
	PrometheusMetricTeamLabel = "workload_namespace"
)

type PrometheusConfig struct {
	EnableFakes bool
	Tenant      string
	Clusters    []string
}

type VulnerabilityPrometheus interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...prom.Option) (prom_model.Value, prom.Warnings, error)
}

func (m *Manager) PromQuery(ctx context.Context, q, cluster string) (prom_model.Vector, error) {
	if m.prometheusClients == nil {
		return nil, fmt.Errorf("no prometheus clients configured")
	}

	prometheusClient, ok := m.prometheusClients[cluster]
	if !ok {
		return nil, fmt.Errorf("no prometheus client configured for cluster: %s", cluster)
	}

	val, _, err := prometheusClient.Query(ctx, q, time.Now())
	if err != nil {
		return nil, err
	}

	if val.Type() != prom_model.ValVector {
		return nil, fmt.Errorf("unexpected PromQuery result type: %s", val.Type())
	}

	if len(val.(prom_model.Vector)) == 0 {
		return nil, nil
	}

	return val.(prom_model.Vector), nil
}

func GetTeamVulnerabilityScore(samples prom_model.Vector, team string, env string) *model.VulnerabilityTeamRank {
	rank := 0
	for _, sample := range samples {
		rank++
		namespace := string(sample.Metric[PrometheusMetricTeamLabel])
		if namespace != team {
			continue
		}

		return &model.VulnerabilityTeamRank{
			Score: int(sample.Value),
			Rank:  rank,
			Env:   env,
		}
	}
	return nil
}
