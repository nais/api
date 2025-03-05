package environment

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/environment/environmentsql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(dbConn))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type dataloader struct {
	db environmentsql.Querier
}

type loaders struct {
	internalQuerier   *environmentsql.Queries
	environmentLoader *dataloadgen.Loader[string, *Environment]
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := environmentsql.New(dbConn)
	environmentLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:   db,
		environmentLoader: dataloadgen.NewLoader(environmentLoader.list, loader.DefaultDataLoaderOptions...),
	}
}

func (l dataloader) list(ctx context.Context, names []string) ([]*Environment, []error) {
	makeKey := func(obj *Environment) string { return obj.Name }
	return loader.LoadModels(ctx, names, l.db.ListByNames, toGraphEnvironment, makeKey)
}

func db(ctx context.Context) *environmentsql.Queries {
	querier := fromContext(ctx).internalQuerier

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return querier.WithTx(tx)
	}

	return querier
}
