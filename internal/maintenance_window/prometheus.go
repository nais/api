package maintenancewindow

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/nais/api/internal/thirdparty/promclient"
	prom "github.com/prometheus/common/model"
)

const (
	PrometheusMetricTeamLabel = "workload_namespace"
)

type PrometheusClient interface {
	Query(ctx context.Context, environment string, query string, opts ...promclient.QueryOption) (prom.Vector, error)
	QueryAll(ctx context.Context, query string, opts ...promclient.QueryOption) (map[string]prom.Vector, error)
}

type PrometheusQuerier struct {
	client PrometheusClient
}

func (p *PrometheusQuerier) riskScoreTotal(ctx context.Context, team string, time time.Time) (float64, error) {
	query := fmt.Sprintf(`sum(slsa_workload_riskscore{workload_namespace="%s"})`, team)
	res, err := p.client.QueryAll(ctx, query, promclient.WithTime(time))
	if err != nil {
		return 0, fmt.Errorf("getting prometheus query result: %w", err)
	}

	total := 0.0
	for _, v := range res {
		for _, sample := range v {
			total += float64(sample.Value)
		}
	}

	return total, nil
}

func (p *PrometheusQuerier) ranking(ctx context.Context, team string, time time.Time) (int, error) {
	query := "sum(slsa_workload_riskscore) by (workload_namespace)"

	res, err := p.client.QueryAll(ctx, query, promclient.WithTime(time))
	if err != nil {
		return 0, fmt.Errorf("getting prometheus query result: %w", err)
	}

	samples := make(prom.Vector, 0)
	for _, v := range res {
		samples = append(samples, v...)
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
