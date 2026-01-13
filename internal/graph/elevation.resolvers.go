package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/elevation"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/user"
)

func (r *elevationResolver) Team(ctx context.Context, obj *elevation.Elevation) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *elevationResolver) TeamEnvironment(ctx context.Context, obj *elevation.Elevation) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *elevationResolver) User(ctx context.Context, obj *elevation.Elevation) (*user.User, error) {
	return user.GetByEmail(ctx, obj.UserEmail)
}

func (r *mutationResolver) CreateElevation(ctx context.Context, input elevation.CreateElevationInput) (*elevation.CreateElevationPayload, error) {
	actor := authz.ActorFromContext(ctx)

	elev, err := elevation.Create(ctx, &input, actor)
	if err != nil {
		return nil, err
	}

	return &elevation.CreateElevationPayload{
		Elevation: elev,
	}, nil
}

func (r *queryResolver) Elevations(ctx context.Context, input elevation.ElevationInput) ([]*elevation.Elevation, error) {
	actor := authz.ActorFromContext(ctx)
	return elevation.List(ctx, &input, actor)
}

func (r *Resolver) Elevation() gengql.ElevationResolver { return &elevationResolver{r} }

type elevationResolver struct{ *Resolver }
