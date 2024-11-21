package api

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/cost/costsql"
	"github.com/nais/api/internal/cost/costupdater"
	"github.com/nais/api/internal/leaderelection"
	"github.com/sirupsen/logrus"
)

const costUpdateSchedule = time.Hour

func costUpdater(ctx context.Context, pool *pgxpool.Pool, cfg *Config, log logrus.FieldLogger) error {
	if !cfg.Cost.ImportEnabled {
		log.Warningf(`cost import is not enabled. Enable by setting the "COST_DATA_IMPORT_ENABLED" environment variable to "true".`)
		return nil
	}

	if err := runCostUpdater(ctx, pool, cfg.Tenant, cfg.Cost.BigQueryProjectID, log.WithField("task", "cost_updater")); err != nil {
		log.WithError(err).Errorf("error in cost updater")
		return err
	}
	return nil
}

// runCostUpdater will create an instance of the cost updater, and update the costs on a schedule. This function will
// block until the context is cancelled, so it should be run in a goroutine.
func runCostUpdater(ctx context.Context, pool *pgxpool.Pool, tenant, bigQueryProjectID string, log logrus.FieldLogger) error {
	updater, err := getUpdater(ctx, pool, tenant, bigQueryProjectID, log)
	if err != nil {
		return fmt.Errorf("unable to set up and run cost updater: %w", err)
	}

	for {
		func() {
			if !leaderelection.IsLeader() {
				log.Debug("not leader, skipping cost update run")
				return
			}

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

			ch := make(chan costsql.CostUpsertParams, costupdater.UpsertBatchSize*2)

			go func() {
				if err := updater.UpdateCosts(ctx, ch); err != nil {
					log.WithError(err).Errorf("failed to update costs")
				}
				done <- struct{}{}
			}()

			if err := updater.FetchBigQueryData(ctx, ch); err != nil {
				log.WithError(err).Errorf("failed to fetch bigquery data")
			}
			close(ch)
			<-done

			if err := updater.RefreshView(ctx); err != nil {
				log.WithError(err).Errorf("unable to refresh cost team monthly")
			}

			log.WithField("duration", time.Since(start)).Infof("cost update run finished")
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(costUpdateSchedule):
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
func getUpdater(ctx context.Context, pool *pgxpool.Pool, tenant, bigQueryProjectID string, log logrus.FieldLogger) (*costupdater.Updater, error) {
	bigQueryClient, err := getBigQueryClient(ctx, bigQueryProjectID)
	if err != nil {
		return nil, err
	}

	return costupdater.NewCostUpdater(
		bigQueryClient,
		costsql.New(pool),
		tenant,
		log.WithField("subsystem", "cost_updater"),
	), nil
}
