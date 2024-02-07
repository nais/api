package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"
)

type Environment struct {
	Name string
	GCP  bool
}

type EnvironmentRepo interface {
	DeleteAllEnvironments(ctx context.Context) error
	InsertEnvironment(ctx context.Context, name string, gcp bool) error
	SyncEnvironments(ctx context.Context, envs []*Environment) error
}

func (d *database) SyncEnvironments(ctx context.Context, envs []*Environment) error {
	return d.Transaction(ctx, func(ctx context.Context, dbtx Database) error {
		if err := dbtx.DeleteAllEnvironments(ctx); err != nil {
			return err
		}

		for _, env := range envs {
			if err := dbtx.InsertEnvironment(ctx, env.Name, env.GCP); err != nil {
				return err
			}
		}

		return nil
	})
}

func (d *database) DeleteAllEnvironments(ctx context.Context) error {
	return d.querier.DeleteAllEnvironments(ctx)
}

func (d *database) InsertEnvironment(ctx context.Context, name string, gcp bool) error {
	return d.querier.InsertEnvironment(ctx, gensql.InsertEnvironmentParams{
		Name: name,
		Gcp:  gcp,
	})
}
