package api

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/environment"
)

func syncEnvironments(ctx context.Context, pool *pgxpool.Pool, clusters ClusterList) error {
	ctx = database.NewLoaderContext(ctx, pool)
	ctx = environment.NewLoaderContext(ctx, pool)

	syncEnvs := make([]*environment.Environment, 0)
	for name, env := range clusters {
		syncEnvs = append(syncEnvs, &environment.Environment{
			Name: name,
			GCP:  env.GCP,
		})
	}

	return environment.SyncEnvironments(ctx, syncEnvs)
}
