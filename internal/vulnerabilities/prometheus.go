package vulnerabilities

import (
	"context"
	"fmt"
	"sort"
	"time"

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

func (m *Manager) promQuery(ctx context.Context, q, cluster string, time time.Time) (prom_model.Vector, error) {
	if m.prometheusClients == nil {
		return nil, fmt.Errorf("no prometheus clients configured")
	}

	prometheusClient, ok := m.prometheusClients[cluster]
	if !ok {
		return nil, fmt.Errorf("no prometheus client configured for cluster: %s", cluster)
	}

	val, _, err := prometheusClient.Query(ctx, q, time)
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

func (m *Manager) ranking(ctx context.Context, team string, time time.Time) (int, error) {
	samples := make(prom_model.Vector, 0)
	for _, e := range m.cfg.Prometheus.Clusters {
		query := "sum(slsa_workload_riskscore) by (workload_namespace)"
		res, err := m.promQuery(ctx, query, e, time)
		if err != nil {
			return 0, fmt.Errorf("getting prometheus query result: %w", err)
		}
		samples = append(samples, res...)
	}

	sort.SliceStable(samples, func(i, j int) bool {
		return samples[i].Value > samples[j].Value
	})

	rank := 0
	for i, s := range samples {
		namespace := string(s.Metric[PrometheusMetricTeamLabel])
		if namespace != team {
			continue
		}

		if rank == 0 {
			rank = i + 1
		}
	}

	return rank, nil
}
