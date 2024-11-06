package cost

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/cost/costsql"
	"github.com/nais/api/internal/database"
)

type ctxKey int

const loadersKey ctxKey = iota

type Option func(l *loaders)

func WithClient(client Client) Option {
	return func(l *loaders) {
		l.client = client
	}
}

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool, opts ...Option) context.Context {
	ldrs := newLoaders(dbConn)
	for _, f := range opts {
		f(ldrs)
	}

	if ldrs.client == nil {
		ldrs.client = &client{}
	}

	return context.WithValue(ctx, loadersKey, ldrs)
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	client          Client
	internalQuerier *costsql.Queries
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := costsql.New(dbConn)

	return &loaders{
		internalQuerier: db,
	}
}

func db(ctx context.Context) *costsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
