package environment

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/environment/environmentsql"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		internalQuerier: environmentsql.New(dbConn),
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier *environmentsql.Queries
}

func db(ctx context.Context) *environmentsql.Queries {
	querier := fromContext(ctx).internalQuerier

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return querier.WithTx(tx)
	}

	return querier
}
