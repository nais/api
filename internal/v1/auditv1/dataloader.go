package auditv1

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/auditv1/auditsql"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
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
	internalQuerier *auditsql.Queries
	auditLogLoader  *dataloadgen.Loader[uuid.UUID, AuditEntry]
}

func newLoaders(dbConn *pgxpool.Pool, opts []dataloadgen.Option) *loaders {
	db := auditsql.New(dbConn)

	auditLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier: db,
		auditLogLoader:  dataloadgen.NewLoader(auditLoader.get, opts...),
	}
}

type dataloader struct {
	db auditsql.Querier
}

func (l dataloader) get(ctx context.Context, ids []uuid.UUID) ([]AuditEntry, []error) {
	makeKey := func(obj AuditEntry) uuid.UUID { return obj.GetUUID() }
	return loaderv1.LoadModelsWithError(ctx, ids, l.db.ListByIDs, toGraphAuditLog, makeKey)
}

func db(ctx context.Context) *auditsql.Queries {
	l := fromContext(ctx)

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
