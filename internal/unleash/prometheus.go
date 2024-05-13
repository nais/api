package unleash

import (
	"context"
	"fmt"
	"time"

	graph "github.com/nais/api/internal/graph/model"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	"github.com/prometheus/common/model"
)

func (m Manager) Metrics(ctx context.Context, unleashInstance *unleash_nais_io_v1.Unleash) (graph.UnleashMetrics, error) {
	metrics := graph.UnleashMetrics{
		NumToggles: 0,
		APITokens:  0,
		Users:      0,
		CpuUsage:   0,
	}

	cpu, err := m.CpuUtilization(ctx, unleashInstance)
	if err != nil {
		return metrics, err
	}

	metrics.CpuUsage = cpu

	return metrics, nil
}

func (m Manager) CpuUtilization(ctx context.Context, unleashInstance *unleash_nais_io_v1.Unleash) (float64, error) {
	query := fmt.Sprintf("irate(process_cpu_user_seconds_total{job=\"%s\", namespace=\"%s\"}[2m])", unleashInstance.Name, m.namespace)
	val, _, err := m.prometheus.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}

	vector := val.(model.Vector)
	fmt.Printf("vector: %v", vector)

	return float64(vector[0].Value) / unleashInstance.Spec.Resources.Requests.Cpu().AsApproximateFloat64() * 100, nil
}
