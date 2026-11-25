package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/team"
)

func (r *mutationResolver) CreatePostgresBackup(ctx context.Context, input postgres.CreatePostgresBackupInput) (*postgres.CreatePostgresBackupPayload, error) {
	return postgres.CreatePostgresBackup(ctx, input)
}

func (r *mutationResolver) DeletePostgresBackup(ctx context.Context, input postgres.DeletePostgresBackupInput) (*postgres.DeletePostgresBackupPayload, error) {
	return postgres.DeletePostgresBackup(ctx, input)
}

func (r *postgresBackupResolver) Team(ctx context.Context, obj *postgres.PostgresBackup) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresBackupResolver) TeamEnvironment(ctx context.Context, obj *postgres.PostgresBackup) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *teamResolver) PostgresBackup(ctx context.Context, obj *team.Team, environmentName string, name string) (*postgres.PostgresBackup, error) {
	return postgres.GetPostgresBackup(ctx, obj.Slug, environmentName, name)
}

func (r *teamResolver) PostgresBackups(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *postgres.PostgresBackupFilter) (*pagination.Connection[*postgres.PostgresBackup], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return postgres.ListPostgresBackupsForTeam(ctx, obj.Slug, page, filter)
}

func (r *Resolver) PostgresBackup() gengql.PostgresBackupResolver { return &postgresBackupResolver{r} }

type postgresBackupResolver struct{ *Resolver }
