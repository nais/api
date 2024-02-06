package loader

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

type teamEnvironmentReader struct {
	db database.TeamRepo
}

func (t teamEnvironmentReader) getEnvironments(ctx context.Context, ids []database.EnvSlugName) ([]*model.Env, []error) {
	getID := func(e *model.Env) database.EnvSlugName {
		return database.EnvSlugName{Slug: e.DBType.TeamSlug, EnvName: e.DBType.Environment}
	}

	return loadModels(ctx, ids, t.db.GetTeamEnvironmentsBySlugsAndEnvNames, ToGraphEnv, getID)
}

func GetTeamEnvironment(ctx context.Context, teamSlug slug.Slug, envName string) (*model.Env, error) {
	return For(ctx).TeamEnvironmentLoader.Load(ctx, database.EnvSlugName{Slug: teamSlug, EnvName: envName})
}

func ToGraphEnv(m *database.TeamEnvironment) *model.Env {
	ret := &model.Env{
		Team:   m.TeamSlug.String(),
		Name:   m.Environment,
		DBType: m,
	}

	return ret
}
