package servicemaintenance

import (
	"context"

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
