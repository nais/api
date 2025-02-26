package search

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/search/searchsql"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool, searcher Client) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(dbConn, searcher))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier *searchsql.Queries
	searcher        Client
}

func newLoaders(dbConn *pgxpool.Pool, searcher Client) *loaders {
	db := searchsql.New(dbConn)

	return &loaders{
		internalQuerier: db,
		searcher:        searcher,
	}
}
