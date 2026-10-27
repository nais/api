package event

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/kubernetes/event/eventsql"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

type identType int

const identKey identType = iota

func init() {
	ident.RegisterIdentType(identKey, "KE", getByIdent)
}

type loaders struct {
	queries eventsql.Querier
	byID    *dataloadgen.Loader[uuid.UUID, WorkloadEvent]
}

func NewLoaderContext(ctx context.Context, pool *pgxpool.Pool) context.Context {
	queries := eventsql.New(pool)
	l := &loaders{queries: queries}
	l.byID = dataloadgen.NewLoader(func(ctx context.Context, ids []uuid.UUID) ([]WorkloadEvent, []error) {
		return loader.LoadModelsWithError(ctx, ids, queries.ListByIDs, toGraphEvent, func(e WorkloadEvent) uuid.UUID { return e.GetUUID() })
	}, loader.DefaultDataLoaderOptions...)
	return context.WithValue(ctx, loadersKey, l)
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

func getByIdent(ctx context.Context, id ident.Ident) (WorkloadEvent, error) {
	key, err := uuid.Parse(id.ID)
	if err != nil {
		return nil, err
	}
	return fromContext(ctx).byID.Load(ctx, key)
}
