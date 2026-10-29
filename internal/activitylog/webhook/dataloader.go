package webhook

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool, dispatcher *Dispatcher) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(dbConn, dispatcher))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier    *webhooksql.Queries
	subscriptionLoader *dataloadgen.Loader[uuid.UUID, *WebhookSubscription]
	deliveryLoader     *dataloadgen.Loader[uuid.UUID, *WebhookDelivery]
	dispatcher         *Dispatcher
}

func newLoaders(dbConn *pgxpool.Pool, dispatcher *Dispatcher) *loaders {
	db := webhooksql.New(dbConn)

	subLoader := &subscriptionDataloader{db: db}
	delLoader := &deliveryDataloader{db: db}

	return &loaders{
		internalQuerier:    db,
		subscriptionLoader: dataloadgen.NewLoader(subLoader.get, loader.DefaultDataLoaderOptions...),
		deliveryLoader:     dataloadgen.NewLoader(delLoader.get, loader.DefaultDataLoaderOptions...),
		dispatcher:         dispatcher,
	}
}

type subscriptionDataloader struct {
	db webhooksql.Querier
}

func (l *subscriptionDataloader) get(ctx context.Context, ids []uuid.UUID) ([]*WebhookSubscription, []error) {
	makeKey := func(obj *WebhookSubscription) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, ids, l.db.ListSubscriptionsByIDs, toGraphSubscription, makeKey)
}

type deliveryDataloader struct {
	db webhooksql.Querier
}

func (l *deliveryDataloader) get(ctx context.Context, ids []uuid.UUID) ([]*WebhookDelivery, []error) {
	makeKey := func(obj *WebhookDelivery) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, ids, l.db.ListDeliveriesByIDs, toGraphDelivery, makeKey)
}

func db(ctx context.Context) *webhooksql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
