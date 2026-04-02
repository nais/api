package agent

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/agent/agentsql"
	"github.com/nais/api/internal/database"
)

type ctxKey int

const loadersKey ctxKey = iota

type loaders struct {
	querier *agentsql.Queries
}

// NewLoaderContext stores agent-specific loaders in the context so that package-level
// query functions can access the database without passing a store explicitly.
func NewLoaderContext(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		querier: agentsql.New(pool),
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

// db returns a transaction-aware querier. If a transaction is active in ctx
// (set by database.Transaction), it will be used; otherwise the base querier is returned.
func db(ctx context.Context) *agentsql.Queries {
	l := fromContext(ctx)
	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.querier.WithTx(tx)
	}
	return l.querier
}
