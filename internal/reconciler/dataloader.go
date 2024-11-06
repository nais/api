package reconciler

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/reconciler/reconcilersql"
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

type loaders struct {
	internalQuerier  *reconcilersql.Queries
	reconcilerLoader *dataloadgen.Loader[string, *Reconciler]
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := reconcilersql.New(dbConn)
	reconcilerLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:  db,
		reconcilerLoader: dataloadgen.NewLoader(reconcilerLoader.list, loader.DefaultDataLoaderOptions...),
	}
}

type dataloader struct {
	db *reconcilersql.Queries
}

func (l dataloader) list(ctx context.Context, names []string) ([]*Reconciler, []error) {
	makeKey := func(obj *Reconciler) string { return obj.Name }
	return loader.LoadModels(ctx, names, l.db.ListByNames, toGraphReconciler, makeKey)
}

func db(ctx context.Context) *reconcilersql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
