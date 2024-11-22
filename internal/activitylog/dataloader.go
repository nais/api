package activitylog

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
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
	internalQuerier   *activitylogsql.Queries
	activityLogLoader *dataloadgen.Loader[uuid.UUID, ActivityLogEntry]
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := activitylogsql.New(dbConn)

	activityLogLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:   db,
		activityLogLoader: dataloadgen.NewLoader(activityLogLoader.get, loader.DefaultDataLoaderOptions...),
	}
}

type dataloader struct {
	db activitylogsql.Querier
}

func (l dataloader) get(ctx context.Context, ids []uuid.UUID) ([]ActivityLogEntry, []error) {
	makeKey := func(obj ActivityLogEntry) uuid.UUID { return obj.GetUUID() }
	return loader.LoadModelsWithError(ctx, ids, l.db.ListByIDs, toGraphActivityLogEntry, makeKey)
}

func db(ctx context.Context) *activitylogsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
