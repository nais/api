package serviceaccount

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/serviceaccount/serviceaccountsql"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(pool))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier             *serviceaccountsql.Queries
	serviceAccountLoader        *dataloadgen.Loader[uuid.UUID, *ServiceAccount]
	serviceAccountTokenLoader   *dataloadgen.Loader[uuid.UUID, *ServiceAccountToken]
	serviceAccountBindingLoader *dataloadgen.Loader[uuid.UUID, *ServiceAccountWorkloadBinding]
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := serviceaccountsql.New(dbConn)
	serviceAccountLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:             db,
		serviceAccountLoader:        dataloadgen.NewLoader(serviceAccountLoader.list, loader.DefaultDataLoaderOptions...),
		serviceAccountTokenLoader:   dataloadgen.NewLoader(serviceAccountLoader.listTokens, loader.DefaultDataLoaderOptions...),
		serviceAccountBindingLoader: dataloadgen.NewLoader(serviceAccountLoader.listBindings, loader.DefaultDataLoaderOptions...),
	}
}

type dataloader struct {
	db *serviceaccountsql.Queries
}

func (l dataloader) list(ctx context.Context, serviceAccountIDs []uuid.UUID) ([]*ServiceAccount, []error) {
	makeKey := func(obj *ServiceAccount) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, serviceAccountIDs, l.db.GetByIDs, toGraphServiceAccount, makeKey)
}

func (l dataloader) listTokens(ctx context.Context, serviceAccountTokenIDs []uuid.UUID) ([]*ServiceAccountToken, []error) {
	makeKey := func(obj *ServiceAccountToken) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, serviceAccountTokenIDs, l.db.GetTokensByIDs, toGraphServiceAccountToken, makeKey)
}

func (l dataloader) listBindings(ctx context.Context, ids []uuid.UUID) ([]*ServiceAccountWorkloadBinding, []error) {
	makeKey := func(obj *ServiceAccountWorkloadBinding) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, ids, l.db.GetBindingsByIDs, toGraphServiceAccountWorkloadBinding, makeKey)
}

func db(ctx context.Context) *serviceaccountsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
