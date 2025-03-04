package graph

import (
	"context"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/team"
)

func (r *queryResolver) Environments(ctx context.Context, orderBy *environment.EnvironmentOrder) ([]*environment.Environment, error) {
	return environment.List(ctx, orderBy)
}

func (r *queryResolver) Environment(ctx context.Context, name string) (*environment.Environment, error) {
	return environment.Get(ctx, name)
}

func (r *teamEnvironmentResolver) Environment(ctx context.Context, obj *team.TeamEnvironment) (*environment.Environment, error) {
	return environment.Get(ctx, obj.EnvironmentName)
}

func (r *Resolver) Environment() gengql.EnvironmentResolver { return &environmentResolver{r} }

type environmentResolver struct{ *Resolver }
