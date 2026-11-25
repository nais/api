package graph

import (
	"context"

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

func (r *teamEnvironmentResolver) PostgresBackup(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.PostgresBackup, error) {
	return postgres.GetPostgresBackup(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamEnvironmentResolver) PostgresBackups(ctx context.Context, obj *team.TeamEnvironment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *postgres.PostgresBackupFilter) (*pagination.Connection[*postgres.PostgresBackup], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return postgres.ListPostgresBackups(ctx, obj.TeamSlug, obj.EnvironmentName, page, filter)
}
