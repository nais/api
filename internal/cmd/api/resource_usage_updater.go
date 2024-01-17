package api

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/resourceusage"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/sirupsen/logrus"
)

func resourceUsageUpdater(ctx context.Context, cfg *Config, db database.Database, k8sClient *k8s.Client, log logrus.FieldLogger) error {
	if !cfg.ResourceUtilizationImportEnabled {
		log.Warningf(`resource utilization import is not enabled. Enable by setting the "RESOURCE_UTILIZATION_IMPORT_ENABLED" environment variable to "true"`)
		return nil
	}

	promClients, err := getPrometheusClients(cfg.K8s.AllClusterNames(), cfg.Tenant)
	if err != nil {
		log.WithError(err).Errorf("create prometheus clients")
		return err
	}

	resourceUsageUpdater := resourceusage.NewUpdater(k8sClient, promClients, db, log)
	if err != nil {
		log.WithError(err).Errorf("create resource usage updater")
		return err
	}

	if err := runResourceUsageUpdater(ctx, resourceUsageUpdater, log.WithField("task", "resource_updater")); err != nil {
		log.WithError(err).Errorf("error in resource usage updater")
	}
	return nil
}

// runResourceUsageUpdater will update resource usage data hourly. This function will block until the context is
// cancelled, so it should be run in a goroutine.
func runResourceUsageUpdater(ctx context.Context, updater *resourceusage.Updater, log logrus.FieldLogger) error {
	ticker := time.NewTicker(time.Second) // initial run
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ticker.Reset(resourceUpdateSchedule) // regular schedule
			start := time.Now()
			log.Infof("start scheduled resource usage update run")
			rows, err := updater.UpdateResourceUsage(ctx)
			if err != nil {
				log = log.WithError(err)
			}
			log.
				WithFields(logrus.Fields{
					"rows_upserted": rows,
					"duration":      time.Since(start),
				}).
				Infof("scheduled resource usage update run finished")
		}
	}
}

// getPrometheusClients will return a map of Prometheus clients, one for each cluster
func getPrometheusClients(clusters []string, tenant string) (map[string]promv1.API, error) {
	promClients := map[string]promv1.API{}
	for _, cluster := range clusters {
		promClient, err := api.NewClient(api.Config{
			Address: fmt.Sprintf("https://prometheus.%s.%s.cloud.nais.io", cluster, tenant),
		})
		if err != nil {
			return nil, err
		}
		promClients[cluster] = promv1.NewAPI(promClient)
	}
	return promClients, nil
}
