package graph

import (
	"context"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
)

func (r *queryResolver) Environments(ctx context.Context, orderBy *environment.EnvironmentOrder) (*pagination.Connection[*environment.Environment], error) {
	envs, err := environment.List(ctx, orderBy)
	if err != nil {
		return nil, err
	}
	return pagination.NewConnectionWithoutPagination(envs), nil
}

func (r *queryResolver) Environment(ctx context.Context, name string) (*environment.Environment, error) {
	return environment.Get(ctx, name)
}

func (r *teamEnvironmentResolver) Environment(ctx context.Context, obj *team.TeamEnvironment) (*environment.Environment, error) {
	return environment.Get(ctx, obj.EnvironmentName)
}

func (r *Resolver) Environment() gengql.EnvironmentResolver { return &environmentResolver{r} }

type environmentResolver struct{ *Resolver }
