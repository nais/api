package graph

import (
	"context"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph/gengql"
)

func (r *environmentResolver) AvailableValues(ctx context.Context, obj *environment.Environment) (*environment.AvailableValues, error) {
	return &environment.AvailableValues{EnvironmentName: obj.Name}, nil
}

func (r *Resolver) AvailableValues() gengql.AvailableValuesResolver {
	return &availableValuesResolver{r}
}

type availableValuesResolver struct{ *Resolver }
