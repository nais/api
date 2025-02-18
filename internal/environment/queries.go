package environment

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/environment/environmentsql"
)

func SyncEnvironments(ctx context.Context, envs []*Environment) error {
	return database.Transaction(ctx, func(ctx context.Context) error {
		if err := deleteAllEnvironments(ctx); err != nil {
			return err
		}

		for _, env := range envs {
			if err := insertEnvironment(ctx, env.Name, env.GCP); err != nil {
				return err
			}
		}

		return nil
	})
}

func deleteAllEnvironments(ctx context.Context) error {
	return db(ctx).DeleteAllEnvironments(ctx)
}

func insertEnvironment(ctx context.Context, name string, gcp bool) error {
	return db(ctx).InsertEnvironment(ctx, environmentsql.InsertEnvironmentParams{
		Name: name,
		Gcp:  gcp,
	})
}
