package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/team"
)

func (r *mutationResolver) GrantPostgresAccess(ctx context.Context, input postgres.GrantPostgresAccessInput) (*postgres.GrantPostgresAccessPayload, error) {
	if err := authz.CanGrantPostgresAccess(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := postgres.GrantZalandoPostgresAccess(ctx, input); err != nil {
		return nil, err
	}

	return &postgres.GrantPostgresAccessPayload{
		Error: new(string),
	}, nil
}

func (r *postgresResolver) Team(ctx context.Context, obj *postgres.Postgres) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresResolver) Environment(ctx context.Context, obj *postgres.Postgres) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *postgresResolver) TeamEnvironment(ctx context.Context, obj *postgres.Postgres) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *Resolver) Postgres() gengql.PostgresResolver { return &postgresResolver{r} }

type postgresResolver struct{ *Resolver }
