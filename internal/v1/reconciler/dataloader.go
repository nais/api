package reconciler

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/reconciler/reconcilersql"
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
	internalQuerier  *reconcilersql.Queries
	reconcilerLoader *dataloadgen.Loader[string, *Reconciler]
}

func newLoaders(dbConn *pgxpool.Pool, opts []dataloadgen.Option) *loaders {
	db := reconcilersql.New(dbConn)
	reconcilerLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:  db,
		reconcilerLoader: dataloadgen.NewLoader(reconcilerLoader.list, opts...),
	}
}

type dataloader struct {
	db *reconcilersql.Queries
}

func (l dataloader) list(ctx context.Context, names []string) ([]*Reconciler, []error) {
	makeKey := func(obj *Reconciler) string { return obj.Name }
	return loaderv1.LoadModels(ctx, names, l.db.ListByNames, toGraphReconciler, makeKey)
}

func db(ctx context.Context) *reconcilersql.Queries {
	l := fromContext(ctx)

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}