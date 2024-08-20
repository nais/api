package event

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/auditv1/auditsql"
	"github.com/nais/api/internal/v1/databasev1"
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
	internalQuerier *auditsql.Queries
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := auditsql.New(dbConn)

	return &loaders{
		internalQuerier: db,
	}
}

func db(ctx context.Context) *auditsql.Queries {
	l := fromContext(ctx)

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
