package usersync

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/usersync/usersyncsql"
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
	internalQuerier   *usersyncsql.Queries
	userSyncLogLoader *dataloadgen.Loader[uuid.UUID, UserSyncLogEntry]
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := usersyncsql.New(dbConn)

	userSyncLogLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:   db,
		userSyncLogLoader: dataloadgen.NewLoader(userSyncLogLoader.get, loader.DefaultDataLoaderOptions...),
	}
}

type dataloader struct {
	db usersyncsql.Querier
}

func (l dataloader) get(ctx context.Context, ids []uuid.UUID) ([]UserSyncLogEntry, []error) {
	makeKey := func(obj UserSyncLogEntry) uuid.UUID { return obj.GetUUID() }
	return loader.LoadModelsWithError(ctx, ids, l.db.ListLogEntriesByIDs, toGraphUserSyncLogEntry, makeKey)
}

func db(ctx context.Context) *usersyncsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
