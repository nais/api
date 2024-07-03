package team

import (
	"context"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamsql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/graphv1/loaderv1"
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
	db         teamsql.Querier
	teamLoader *dataloadgen.Loader[slug.Slug, *Team]
}

func newLoaders(dbConn *pgxpool.Pool, opts []dataloadgen.Option) *loaders {
	db := teamsql.New(dbConn)
	teamLoader := &dataloader{db: db}

	return &loaders{
		db:         db,
		teamLoader: dataloadgen.NewLoader(teamLoader.list, opts...),
	}
}

type dataloader struct {
	db teamsql.Querier
}

func (l dataloader) list(ctx context.Context, slugs []slug.Slug) ([]*Team, []error) {
	getID := func(obj *Team) slug.Slug { return obj.Slug }
	return loaderv1.LoadModels(ctx, slugs, l.db.GetBySlugs, toGraphTeam, getID)
}
