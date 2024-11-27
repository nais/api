package graph

import (
	"context"

	"github.com/nais/api/internal/feature"
)

func (r *queryResolver) Features(ctx context.Context) (*feature.Features, error) {
	return feature.Get(ctx)
}
