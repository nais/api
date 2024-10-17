package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/github/repository/repositorysql"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(dbConn))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier *repositorysql.Queries
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	return &loaders{
		internalQuerier: repositorysql.New(dbConn),
	}
}

func db(ctx context.Context) *repositorysql.Queries {
	l := fromContext(ctx)

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
