package audit

import (
	"context"

	"github.com/nais/api/internal/graph/loader"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/audit/auditsql"
	"github.com/nais/api/internal/database"
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
	internalQuerier *auditsql.Queries
	auditLogLoader  *dataloadgen.Loader[uuid.UUID, AuditEntry]
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := auditsql.New(dbConn)

	auditLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier: db,
		auditLogLoader:  dataloadgen.NewLoader(auditLoader.get, loader.DefaultDataLoaderOptions...),
	}
}

type dataloader struct {
	db auditsql.Querier
}

func (l dataloader) get(ctx context.Context, ids []uuid.UUID) ([]AuditEntry, []error) {
	makeKey := func(obj AuditEntry) uuid.UUID { return obj.GetUUID() }
	return loader.LoadModelsWithError(ctx, ids, l.db.ListByIDs, toGraphAuditLog, makeKey)
}

func db(ctx context.Context) *auditsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}