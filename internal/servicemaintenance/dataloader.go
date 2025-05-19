package servicemaintenance

import (
	"context"

	aiven_service "github.com/aiven/go-client-codegen/handler/service"
	"github.com/nais/api/internal/graph/loader"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, serviceMaintenanceManager *Manager, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(serviceMaintenanceManager, logger))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type AivenDataLoaderKey struct {
	Project     string
	ServiceName string
}

type loaders struct {
	maintenanceLoader  *dataloadgen.Loader[*AivenDataLoaderKey, []aiven_service.UpdateOut]
	log                logrus.FieldLogger
	maintenanceMutator *Manager
}

func newLoaders(serviceMaintenanceMgr *Manager, logger logrus.FieldLogger) *loaders {
	maintenanceLoader := &dataloader{serviceMaintenanceManager: serviceMaintenanceMgr, log: logger}
	return &loaders{
		maintenanceLoader:  dataloadgen.NewLoader(maintenanceLoader.aivenMaintenanceList, loader.DefaultDataLoaderOptions...),
		log:                logger,
		maintenanceMutator: serviceMaintenanceMgr,
	}
}

type dataloader struct {
	serviceMaintenanceManager *Manager
	log                       logrus.FieldLogger
}

func (l dataloader) aivenMaintenanceList(ctx context.Context, aivenDataLoaderKeys []*AivenDataLoaderKey) ([][]aiven_service.UpdateOut, []error) {
	wg := pool.New().WithContext(ctx)
	rets := make([][]aiven_service.UpdateOut, len(aivenDataLoaderKeys))
	errs := make([]error, len(aivenDataLoaderKeys))

	for i, pair := range aivenDataLoaderKeys {
		wg.Go(func(ctx context.Context) error {
			res, err := l.serviceMaintenanceManager.aivenClient.ServiceGet(ctx, pair.Project, pair.ServiceName)
			if err != nil {
				errs[i] = err
			} else {
				if res.Maintenance != nil && res.Maintenance.Updates != nil {
					rets[i] = res.Maintenance.Updates
				}
			}
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		l.log.WithError(err).Error("error waiting for dataloader")
	}

	return rets, errs
}
