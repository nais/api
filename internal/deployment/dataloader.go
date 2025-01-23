package deployment

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/deployment/deploymentsql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, pool *pgxpool.Pool, client hookd.Client) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(pool, client))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier  *deploymentsql.Queries
	deploymentLoader *dataloadgen.Loader[uuid.UUID, *Deployment]
	client           hookd.Client
}

func newLoaders(pool *pgxpool.Pool, client hookd.Client) *loaders {
	db := deploymentsql.New(pool)
	deploymentLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:  db,
		deploymentLoader: dataloadgen.NewLoader(deploymentLoader.list, loader.DefaultDataLoaderOptions...),
		client:           client,
	}
}

type dataloader struct {
	db *deploymentsql.Queries
}

func (l dataloader) list(ctx context.Context, ids []uuid.UUID) ([]*Deployment, []error) {
	makeKey := func(obj *Deployment) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, ids, l.db.ListByIDs, toGraphDeployment, makeKey)
}

func db(ctx context.Context) *deploymentsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
