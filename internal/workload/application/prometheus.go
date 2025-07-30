package application

import (
	"context"

	"github.com/nais/api/internal/thirdparty/promclient"
	prom "github.com/prometheus/common/model"
)

type IngressMetricsClient interface {
	Query(ctx context.Context, environment string, query string, opts ...promclient.QueryOption) (prom.Vector, error)
	// QueryAll(ctx context.Context, query string, opts ...promclient.QueryOption) (map[string]prom.Vector, error)
	// QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error)
}

func RequestsPerSecondForIngress(ctx context.Context) float64 {
	_ = fromContext(ctx).client

	// c.Query(ctx, obj.EnvironmentName, "rate(ingress_requests_total[1m])", promclient.WithLegend("app", obj.Name))
	return 0.0
}

func ErrorsPerSecondForIngress(ctx context.Context) float64 {
	return 0.0
}
