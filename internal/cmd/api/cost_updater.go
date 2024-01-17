package api

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/nais/api/internal/cost"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/sirupsen/logrus"
)

func costUpdater(ctx context.Context, cfg *Config, db database.Database, log logrus.FieldLogger) error {
	if !cfg.Cost.ImportEnabled {
		log.Warningf(`cost import is not enabled. Enable by setting the "COST_DATA_IMPORT_ENABLED" environment variable to "true".`)
		return nil
	}

	err := runCostUpdater(ctx, db, cfg.Tenant, cfg.Cost.BigQueryProjectID, log.WithField("task", "cost_updater"))
	if err != nil {
		log.WithError(err).Errorf("error in cost updater")
		return err
	}
	return nil
}

// runCostUpdater will create an instance of the cost updater, and update the costs on a schedule. This function will
// block until the context is cancelled, so it should be run in a goroutine.
func runCostUpdater(ctx context.Context, db database.Database, tenant, bigQueryProjectID string, log logrus.FieldLogger) error {
	updater, err := getUpdater(ctx, db, tenant, bigQueryProjectID, log)
	if err != nil {
		return fmt.Errorf("unable to set up and run cost updater: %w", err)
	}

	ticker := time.NewTicker(1 * time.Second) // initial run
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			func() {
				ticker.Reset(costUpdateSchedule) // regular schedule
				log.Infof("start scheduled cost update run")
				start := time.Now()

				if shouldUpdate, err := updater.ShouldUpdateCosts(ctx); err != nil {
					log.WithError(err).Errorf("unable to check if costs should be updated")
					return
				} else if !shouldUpdate {
					log.Infof("no need to update costs yet")
					return
				}

				ctx, cancel := context.WithTimeout(ctx, costUpdateSchedule-5*time.Minute)
				defer cancel()

				done := make(chan struct{})
				defer close(done)

				ch := make(chan gensql.CostUpsertParams, cost.UpsertBatchSize*2)

				go func() {
					err := updater.UpdateCosts(ctx, ch)
					if err != nil {
						log.WithError(err).Errorf("failed to update costs")
					}
					done <- struct{}{}
				}()

				err = updater.FetchBigQueryData(ctx, ch)
				if err != nil {
					log.WithError(err).Errorf("failed to fetch bigquery data")
				}
				close(ch)
				<-done

				log.WithFields(logrus.Fields{
					"duration": time.Since(start),
				}).Infof("cost update run finished")
			}()
		}
	}
}

// getBigQueryClient will return a new BigQuery client for the specified project
func getBigQueryClient(ctx context.Context, projectID string) (*bigquery.Client, error) {
	bigQueryClient, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	bigQueryClient.Location = "EU"
	return bigQueryClient, nil
}

// getBigQueryClient will return a new cost updater instance
func getUpdater(ctx context.Context, db database.Database, tenant, bigQueryProjectID string, log logrus.FieldLogger) (*cost.Updater, error) {
	bigQueryClient, err := getBigQueryClient(ctx, bigQueryProjectID)
	if err != nil {
		return nil, err
	}

	return cost.NewCostUpdater(
		bigQueryClient,
		db,
		tenant,
		log.WithField("subsystem", "cost_updater"),
	), nil
}
