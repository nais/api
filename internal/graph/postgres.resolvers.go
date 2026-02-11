package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) PostgresInstances(ctx context.Context, obj *application.Application, orderBy *postgres.PostgresInstanceOrder) (*pagination.Connection[*postgres.PostgresInstance], error) {
	panic(fmt.Errorf("not implemented: PostgresInstances - postgresInstances"))
}

func (r *jobResolver) PostgresInstances(ctx context.Context, obj *job.Job, orderBy *postgres.PostgresInstanceOrder) (*pagination.Connection[*postgres.PostgresInstance], error) {
	panic(fmt.Errorf("not implemented: PostgresInstances - postgresInstances"))
}

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

func (r *postgresInstanceResolver) Team(ctx context.Context, obj *postgres.PostgresInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresInstanceResolver) Environment(ctx context.Context, obj *postgres.PostgresInstance) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *postgresInstanceResolver) TeamEnvironment(ctx context.Context, obj *postgres.PostgresInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *teamResolver) PostgresInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *postgres.PostgresInstanceOrder) (*pagination.Connection[*postgres.PostgresInstance], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return postgres.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) PostgresInstances(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.PostgresInstance, error) {
	panic(fmt.Errorf("not implemented: PostgresInstances - postgresInstances"))
}

func (r *Resolver) PostgresInstance() gengql.PostgresInstanceResolver {
	return &postgresInstanceResolver{r}
}

type postgresInstanceResolver struct{ *Resolver }
