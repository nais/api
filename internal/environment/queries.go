package environment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/environment/environmentsql"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
)

func Get(ctx context.Context, name string) (*Environment, error) {
	e, err := fromContext(ctx).environmentLoader.Load(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apierror.Errorf("Environment %q not found", name)
	} else if err != nil {
		return nil, err
	}

	return e, nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Environment, error) {
	name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, name)
}

func List(ctx context.Context, orderBy *EnvironmentOrder) ([]*Environment, error) {
	rows, err := db(ctx).List(ctx)
	if err != nil {
		return nil, err
	}

	envs := make([]*Environment, len(rows))
	for i, row := range rows {
		envs[i] = toGraphEnvironment(row)
	}

	if orderBy == nil {
		orderBy = &EnvironmentOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilter.Sort(ctx, envs, orderBy.Field, orderBy.Direction)
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
