package serviceaccount

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/serviceaccount/serviceaccountsql"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		internalQuerier: serviceaccountsql.New(pool),
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier *serviceaccountsql.Queries
}

func db(ctx context.Context) *serviceaccountsql.Queries {
	l := fromContext(ctx)

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
