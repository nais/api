package servicemaintenance

import (
	"context"
	"time"

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

type AivenDataLoaderKey struct {
	Project     string
	ServiceName string
}

type loaders struct {
	maintenanceLoader  *dataloadgen.Loader[*AivenDataLoaderKey, *ServiceMaintenance]
	maintenanceMutator *Manager
}

func newLoaders(serviceMaintenanceMgr *Manager, logger logrus.FieldLogger) *loaders {
	maintenanceLoader := &dataloader{serviceMaintenanceManager: serviceMaintenanceMgr, log: logger}
	return &loaders{
		maintenanceLoader:  dataloadgen.NewLoader(maintenanceLoader.maintenanceList, loader.DefaultDataLoaderOptions...),
		maintenanceMutator: serviceMaintenanceMgr,
	}
}

type dataloader struct {
	serviceMaintenanceManager *Manager
	log                       logrus.FieldLogger
}

func (l dataloader) maintenanceList(ctx context.Context, aivenDataLoaderKeys []*AivenDataLoaderKey) ([]*ServiceMaintenance, []error) {
	wg := pool.New().WithContext(ctx)
	rets := make([]*ServiceMaintenance, len(aivenDataLoaderKeys))
	errs := make([]error, len(aivenDataLoaderKeys))

	for i, pair := range aivenDataLoaderKeys {
		wg.Go(func(ctx context.Context) error {
			res, err := l.serviceMaintenanceManager.client.ServiceGet(ctx, pair.Project, pair.ServiceName)
			if err != nil {
				errs[i] = err
			} else {
				if res.Maintenance != nil && res.Maintenance.Updates != nil {
					updates := make([]ServiceMaintenanceUpdate, len(res.Maintenance.Updates))
					for j, update := range res.Maintenance.Updates {
						updates[j] = ServiceMaintenanceUpdate{
							Title:             *update.Description,
							Description:       *update.Impact,
							DocumentationLink: update.DocumentationLink,
							StartAt:           update.StartAt,
						}

						if update.Deadline != nil {
							if t, err := time.Parse(time.RFC3339, *update.Deadline); err == nil {
								updates[j].Deadline = &t
							} else {
								l.log.WithError(err).Warnf("Failed to parse deadline time: %v", update.Deadline)
							}
						}
					}
					rets[i] = &ServiceMaintenance{Updates: updates}
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
