package team

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/team/teamsql"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool, defaultOpts []dataloadgen.Option) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(dbConn, defaultOpts))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier       *teamsql.Queries
	teamLoader            *dataloadgen.Loader[slug.Slug, *Team]
	teamEnvironmentLoader *dataloadgen.Loader[envSlugName, *TeamEnvironment]
}

func newLoaders(dbConn *pgxpool.Pool, opts []dataloadgen.Option) *loaders {
	db := teamsql.New(dbConn)
	teamLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:       db,
		teamLoader:            dataloadgen.NewLoader(teamLoader.list, opts...),
		teamEnvironmentLoader: dataloadgen.NewLoader(teamLoader.getEnvironments, opts...),
	}
}

type dataloader struct {
	db teamsql.Querier
}

func (l dataloader) list(ctx context.Context, slugs []slug.Slug) ([]*Team, []error) {
	makeKey := func(obj *Team) slug.Slug { return obj.Slug }
	return loaderv1.LoadModels(ctx, slugs, l.db.ListBySlugs, toGraphTeam, makeKey)
}

func (l dataloader) getEnvironments(ctx context.Context, ids []envSlugName) ([]*TeamEnvironment, []error) {
	makeKey := func(e *TeamEnvironment) envSlugName {
		return envSlugName{Slug: e.TeamSlug, EnvName: e.Name}
	}

	return loaderv1.LoadModels(ctx, ids, l.getTeamEnvironmentsBySlugsAndEnvNames, toGraphTeamEnvironment, makeKey)
}

func (l dataloader) getTeamEnvironmentsBySlugsAndEnvNames(ctx context.Context, lookup []envSlugName) ([]*teamsql.TeamAllEnvironment, error) {
	slugs := make([]slug.Slug, len(lookup))
	envNames := make([]string, len(lookup))

	for i, v := range lookup {
		slugs[i] = v.Slug
		envNames[i] = v.EnvName
	}

	return l.db.ListEnvironmentsBySlugsAndEnvNames(ctx, teamsql.ListEnvironmentsBySlugsAndEnvNamesParams{
		TeamSlugs:    slugs,
		Environments: envNames,
	})
}

type envSlugName struct {
	Slug    slug.Slug
	EnvName string
}

func db(ctx context.Context) *teamsql.Queries {
	l := fromContext(ctx)

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
