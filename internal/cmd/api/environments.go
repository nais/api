package api

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/environmentmapper"
)

func syncEnvironments(ctx context.Context, pool *pgxpool.Pool, clusters ClusterList, oidcIssuers middleware.KubernetesIssuers) error {
	ctx = database.NewLoaderContext(ctx, pool)
	ctx = environment.NewLoaderContext(ctx, pool)

	issuerByEnv := make(map[string]string, len(oidcIssuers))
	for _, iss := range oidcIssuers {
		issuerByEnv[iss.Environment] = iss.Issuer
	}

	syncEnvs := make([]*environment.Environment, 0, len(clusters))
	for name, env := range clusters {
		envName := environmentmapper.EnvironmentName(name)
		e := &environment.Environment{
			Name: envName,
			GCP:  env.GCP,
		}
		if issuer, ok := issuerByEnv[envName]; ok {
			e.OIDCIssuerURL = &issuer
		}
		syncEnvs = append(syncEnvs, e)
	}

	return environment.SyncEnvironments(ctx, syncEnvs)
}
