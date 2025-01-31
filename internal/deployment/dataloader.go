package deployment

import (
	"context"
	"time"

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
	internalQuerier                          *deploymentsql.Queries
	deploymentLoader                         *dataloadgen.Loader[uuid.UUID, *Deployment]
	deploymentResourceLoader                 *dataloadgen.Loader[uuid.UUID, *DeploymentResource]
	deploymentStatusLoader                   *dataloadgen.Loader[uuid.UUID, *DeploymentStatus]
	deploymentStatusForDeploymentCountLoader *dataloadgen.Loader[uuid.UUID, *deploymentStatusForDeploymentCount]
	client                                   hookd.Client
}

func newLoaders(pool *pgxpool.Pool, client hookd.Client) *loaders {
	db := deploymentsql.New(pool)
	deploymentLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:          db,
		deploymentLoader:         dataloadgen.NewLoader(deploymentLoader.list, loader.DefaultDataLoaderOptions...),
		deploymentResourceLoader: dataloadgen.NewLoader(deploymentLoader.listDeploymentResources, loader.DefaultDataLoaderOptions...),
		deploymentStatusLoader:   dataloadgen.NewLoader(deploymentLoader.listDeploymentStatuses, loader.DefaultDataLoaderOptions...),
		deploymentStatusForDeploymentCountLoader: dataloadgen.NewLoader(
			deploymentLoader.countDeploymentStatusForDeployment,
			append(loader.DefaultDataLoaderOptions, dataloadgen.WithWait(3*time.Millisecond))...,
		),
		client: client,
	}
}

type dataloader struct {
	db *deploymentsql.Queries
}

func (l dataloader) list(ctx context.Context, ids []uuid.UUID) ([]*Deployment, []error) {
	makeKey := func(obj *Deployment) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, ids, l.db.ListByIDs, toGraphDeployment, makeKey)
}

func (l dataloader) listDeploymentResources(ctx context.Context, ids []uuid.UUID) ([]*DeploymentResource, []error) {
	makeKey := func(obj *DeploymentResource) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, ids, l.db.ListDeploymentResourcesByIDs, toGraphDeploymentResource, makeKey)
}

func (l dataloader) listDeploymentStatuses(ctx context.Context, ids []uuid.UUID) ([]*DeploymentStatus, []error) {
	makeKey := func(obj *DeploymentStatus) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, ids, l.db.ListDeploymentStatusesByIDs, toGraphDeploymentStatus, makeKey)
}

func (l dataloader) countDeploymentStatusForDeployment(ctx context.Context, ids []uuid.UUID) ([]*deploymentStatusForDeploymentCount, []error) {
	makeKey := func(obj *deploymentStatusForDeploymentCount) uuid.UUID { return obj.DeploymentID }
	return loader.LoadModels(ctx, ids, l.db.CountStatusesForDeploymentIDs, func(d *deploymentsql.CountStatusesForDeploymentIDsRow) *deploymentStatusForDeploymentCount {
		return &deploymentStatusForDeploymentCount{
			DeploymentID: d.DeploymentID,
			Count:        d.Count,
		}
	}, makeKey)
}

func db(ctx context.Context) *deploymentsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
