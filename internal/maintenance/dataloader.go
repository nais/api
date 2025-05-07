package maintenance

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

func NewLoaderContext(ctx context.Context, prometheusClient PrometheusClient, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(prometheusClient, logger))
}

type aivenDataLoaderKey struct {
	project     string
	serviceName string
}

type loaders struct {
	maintenanceLoader  *dataloadgen.Loader[*aivenDataLoaderKey, *Maintenance]
	maintenanceManager *Manager
	promClients        *PrometheusQuerier
}

func newLoaders(prometheusClient PrometheusClient, logger logrus.FieldLogger) *loaders {
	maintenanceLoader := &dataloader{maintenanceManager: nil, log: logger}

	return &loaders{
		maintenanceLoader: dataloadgen.NewLoader(maintenanceLoader.maintenanceList, loader.DefaultDataLoaderOptions...),
		promClients: &PrometheusQuerier{
			client: prometheusClient,
		},
	}
}

type dataloader struct {
	maintenanceManager *Manager
	log                logrus.FieldLogger
}

func (l dataloader) maintenanceList(ctx context.Context, aivenDataLoaderKeys []*aivenDataLoaderKey) ([]*Maintenance, []error) {
	wg := pool.New().WithContext(ctx)

	rets := make([]*Maintenance, len(aivenDataLoaderKeys))
	errs := make([]error, len(aivenDataLoaderKeys))

	for i, pair := range aivenDataLoaderKeys {
		wg.Go(func(ctx context.Context) error {
			res, err := l.maintenanceManager.client.ServiceGet(ctx, pair.project, pair.serviceName)
			if err != nil {
				errs[i] = err
			} else {
				if res.Maintenance != nil && res.Maintenance.Updates != nil {
					updates := make([]Update, len(res.Maintenance.Updates))
					for j, update := range res.Maintenance.Updates {
						updates[j] = Update{
							Title:             *update.Description,
							Description:       *update.Impact,
							DocumentationLink: *update.DocumentationLink,
							StartAt:           update.StartAt,
						}
						if update.Deadline != nil {
							if t, err := time.Parse(time.RFC3339, *update.Deadline); err == nil {
								updates[j].Deadline = &t
							} else {
								l.log.WithError(err).Warnf("Failed to parse deadline time: %v", update.Deadline)
							}
						}

						if update.StartAfter != nil {
							if t, err := time.Parse(time.RFC3339, *update.StartAfter); err == nil {
								updates[j].StartAfter = &t
							} else {
								l.log.WithError(err).Warnf("Failed to parse start_after time: %v", update.StartAfter)
							}
						}

					}
					rets[i] = &Maintenance{Updates: updates}
				}
			}
			return nil
		})
	}
	return rets, errs
}
