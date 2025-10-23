package graph

import (
	"context"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/metrics"
)

func (r *environmentResolver) Metrics(ctx context.Context, obj *environment.Environment, input metrics.MetricsQueryInput) (*metrics.MetricsQueryResult, error) {
	return metrics.Query(ctx, input, obj.Name)
}
