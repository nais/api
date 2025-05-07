package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/price"
)

func (r *currentUnitPricesResolver) CPU(ctx context.Context, obj *price.CurrentUnitPrices) (*price.Price, error) {
	return price.CPUHour(ctx)
}

func (r *currentUnitPricesResolver) Memory(ctx context.Context, obj *price.CurrentUnitPrices) (*price.Price, error) {
	p, err := price.MemoryHour(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *queryResolver) CurrentUnitPrices(ctx context.Context) (*price.CurrentUnitPrices, error) {
	return &price.CurrentUnitPrices{}, nil
}

func (r *Resolver) CurrentUnitPrices() gengql.CurrentUnitPricesResolver {
	return &currentUnitPricesResolver{r}
}

type currentUnitPricesResolver struct{ *Resolver }
