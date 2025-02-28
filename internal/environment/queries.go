package environment

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/environment/environmentsql"
)

func GetByName(ctx context.Context, name string) (*Environment, error) {
	e, err := db(ctx).Get(ctx, name)
	if err != nil {
		return nil, err
	}

	return toGraphEnvironment(e), nil
}

func List(ctx context.Context) ([]*Environment, error) {
	rows, err := db(ctx).List(ctx)
	if err != nil {
		return nil, err
	}

	envs := make([]*Environment, len(rows))
	for i, row := range rows {
		envs[i] = toGraphEnvironment(row)
	}

	return envs, nil
}

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
