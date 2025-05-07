package graph

import (
	"context"

	"github.com/nais/api/internal/price"
)

func (r *queryResolver) CurrentUnitPrice(ctx context.Context, resourceType price.ResourceType) (*price.Price, error) {
	return price.GetPrice(ctx, resourceType)
}
