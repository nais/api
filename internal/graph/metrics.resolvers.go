package graph

import (
	"context"

	"github.com/nais/api/internal/metrics"
)

func (r *queryResolver) Metrics(ctx context.Context, input metrics.MetricsQueryInput) (*metrics.MetricsQueryResult, error) {
	return metrics.Query(ctx, input)
}
