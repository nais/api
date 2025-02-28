package graph

import (
	"context"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/team"
)

func (r *queryResolver) Environments(ctx context.Context) ([]*environment.Environment, error) {
	return environment.List(ctx)
}

func (r *teamEnvironmentResolver) Environment(ctx context.Context, obj *team.TeamEnvironment) (*environment.Environment, error) {
	return environment.GetByName(ctx, obj.Name)
}
