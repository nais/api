package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/persistence/zalandopostgres"
	"github.com/nais/api/internal/team"
)

func (r *mutationResolver) GrantZalandoPostgresAccess(ctx context.Context, input zalandopostgres.GrantZalandoPostgresAccessInput) (*zalandopostgres.GrantZalandoPostgresAccessPayload, error) {
	if err := authz.CanGrantPostgresAccess(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := zalandopostgres.GrantZalandoPostgresAccess(ctx, input); err != nil {
		return nil, err
	}

	return &zalandopostgres.GrantZalandoPostgresAccessPayload{
		Error: new(string),
	}, nil
}

func (r *zalandoPostgresResolver) Team(ctx context.Context, obj *zalandopostgres.ZalandoPostgres) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *zalandoPostgresResolver) Environment(ctx context.Context, obj *zalandopostgres.ZalandoPostgres) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *zalandoPostgresResolver) TeamEnvironment(ctx context.Context, obj *zalandopostgres.ZalandoPostgres) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *Resolver) ZalandoPostgres() gengql.ZalandoPostgresResolver {
	return &zalandoPostgresResolver{r}
}

type zalandoPostgresResolver struct{ *Resolver }
