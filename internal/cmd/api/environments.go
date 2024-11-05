package api

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/environment"
)

func syncEnvironments(ctx context.Context, pool *pgxpool.Pool, clusters ClusterList) error {
	ctx = databasev1.NewLoaderContext(ctx, pool)
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
