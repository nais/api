package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/sqlinstance"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, k8sClient *k8s.Client, sqlAdminService *sqlinstance.SqlAdminService, defaultOpts []dataloadgen.Option) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(k8sClient, sqlAdminService, defaultOpts))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	k8sClient       *client
	sqlAdminService *sqlinstance.SqlAdminService
	instanceLoader  *dataloadgen.Loader[identifier, *SQLInstance]
	databaseLoader  *dataloadgen.Loader[identifier, *SQLDatabase]
}

func newLoaders(k8sClient *k8s.Client, sqlAdminService *sqlinstance.SqlAdminService, opts []dataloadgen.Option) *loaders {
	client := &client{
		informers: k8sClient.Informers(),
	}

	instanceLoader := &dataloader{
		k8sClient: client,
	}
	databaseLoader := &dataloader{
		k8sClient: client,
	}

	return &loaders{
		k8sClient:       client,
		sqlAdminService: sqlAdminService,
		instanceLoader:  dataloadgen.NewLoader(instanceLoader.listInstances, opts...),
		databaseLoader:  dataloadgen.NewLoader(databaseLoader.listDatabases, opts...),
	}
}

type dataloader struct {
	k8sClient *client
}

type identifier struct {
	namespace       string
	environmentName string
	sqlInstanceName string
}

func (l dataloader) listInstances(ctx context.Context, ids []identifier) ([]*SQLInstance, []error) {
	makeKey := func(obj *SQLInstance) identifier {
		return identifier{
			namespace:       obj.TeamSlug.String(),
			environmentName: obj.EnvironmentName,
			sqlInstanceName: obj.Name,
		}
	}
	return loaderv1.LoadModels(ctx, ids, l.k8sClient.getInstances, func(d *SQLInstance) *SQLInstance { return d }, makeKey)
}

func (l dataloader) listDatabases(ctx context.Context, ids []identifier) ([]*SQLDatabase, []error) {
	makeKey := func(obj *SQLDatabase) identifier {
		return identifier{
			namespace:       obj.TeamSlug.String(),
			environmentName: obj.EnvironmentName,
			sqlInstanceName: obj.SQLInstanceName,
		}
	}
	return loaderv1.LoadModels(ctx, ids, l.k8sClient.getDatabases, func(d *SQLDatabase) *SQLDatabase { return d }, makeKey)
}
