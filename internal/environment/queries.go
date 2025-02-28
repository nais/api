package environment

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/environment/environmentsql"
	"github.com/nais/api/internal/graph/ident"
)

func Get(ctx context.Context, name string) (*Environment, error) {
	return fromContext(ctx).environmentLoader.Load(ctx, name)
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Environment, error) {
	name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, name)
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
