package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/cost"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/fixtures"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/dataloader"
	"github.com/nais/api/internal/graph/directives"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/k8s/fake"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/resourceusage"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/dependencytrack"
	faketrack "github.com/nais/api/internal/thirdparty/dependencytrack/fake"
	"github.com/nais/api/internal/thirdparty/hookd"
	fakehookd "github.com/nais/api/internal/thirdparty/hookd/fake"
	"github.com/nais/api/internal/usersync"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/exporters/prometheus"
	met "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"golang.org/x/oauth2/google"
)

const (
	exitCodeSuccess = iota
	exitCodeLoggerError
	exitCodeRunError
	exitCodeConfigError
	exitCodeEnvFileError
)

const (
	costUpdateSchedule     = time.Hour
	resourceUpdateSchedule = time.Hour
	userSyncInterval       = time.Minute * 15
	userSyncTimeout        = time.Second * 30
)

func main() {
	ctx := context.Background()
	log := logrus.StandardLogger()

	if fileLoaded, err := loadEnvFile(); err != nil {
		log.WithError(err).Errorf("error when loading .env file")
		os.Exit(exitCodeEnvFileError)
	} else if fileLoaded {
		log.Infof("loaded .env file")
	}

	cfg, err := NewConfig(ctx, envconfig.OsLookuper())
	if err != nil {
		log.WithError(err).Errorf("error when processing configuration")
		os.Exit(exitCodeConfigError)
	}

	appLogger, err := logger.New(cfg.LogFormat, cfg.LogLevel)
	if err != nil {
		log.WithError(err).Errorf("error when creating application logger")
		os.Exit(exitCodeLoggerError)
	}

	err = run(ctx, cfg, appLogger)
	if err != nil {
		appLogger.WithError(err).Errorf("error in run()")
		os.Exit(exitCodeRunError)
	}

	os.Exit(exitCodeSuccess)
}

func run(ctx context.Context, cfg *Config, log logrus.FieldLogger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// TODO: Replace with signal.NotifyContext
	signals := make(chan os.Signal, 1)
	defer close(signals)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	if cfg.WithFakeClients {
		log.Warn("using fake clients")
	}

	meter, err := getMetricMeter()
	if err != nil {
		return fmt.Errorf("create metric meter: %w", err)
	}

	errorsCounter, err := meter.Int64Counter("errors")
	if err != nil {
		return fmt.Errorf("create error counter: %w", err)
	}

	log.Info("connecting to database")
	db, closer, err := database.New(ctx, cfg.DatabaseConnectionString, log.WithField("subsystem", "database"))
	if err != nil {
		return fmt.Errorf("setting up database: %w", err)
	}
	defer closer()

	firstRun, err := db.IsFirstRun(ctx)
	if err != nil {
		return err
	}
	if firstRun {
		log.Infof("first run detected ")
		firstRunLogger := log.WithField("system", "first-run")
		if err := fixtures.SetupDefaultReconcilers(ctx, firstRunLogger, cfg.FirstRunEnableReconcilers, db); err != nil {
			return err
		}

		if err := db.FirstRunComplete(ctx); err != nil {
			return err
		}
	}

	err = fixtures.SetupStaticServiceAccounts(ctx, db, cfg.StaticServiceAccounts)
	if err != nil {
		return err
	}

	k8sOpts := []k8s.Opt{}
	if cfg.WithFakeClients {
		k8sOpts = append(k8sOpts, k8s.WithClientsCreator(fake.Clients(os.DirFS("./data/k8s"))))
	}

	k8sClient, err := k8s.New(
		cfg.Tenant,
		cfg.K8s.PkgConfig(),
		errorsCounter,
		&teamChecker{db: db},
		log.WithField("client", "k8s"),
		k8sOpts...,
	)
	if err != nil {
		var authErr *google.AuthenticationError
		if errors.As(err, &authErr) {
			return fmt.Errorf("unable to create k8s client. You should probably run `gcloud auth login --update-adc` and authenticate with your @nais.io-account before starting api: %w", err)
		}
		return fmt.Errorf("unable to create k8s client: %w", err)
	}

	clusters := make(graph.ClusterList)
	for _, cluster := range cfg.K8s.Clusters {
		clusters[cluster] = graph.ClusterInfo{
			GCP: true,
		}
	}
	for _, staticCluster := range cfg.K8s.StaticClusters {
		clusters[staticCluster.Name] = graph.ClusterInfo{}
	}

	auditLogger := auditlogger.New(db, logger.ComponentNameGraphqlApi, log)
	userSync := make(chan uuid.UUID, 1)

	var hookdClient graph.HookdClient
	var dependencyTrackClient graph.DependencytrackClient
	if cfg.WithFakeClients {
		hookdClient = fakehookd.New()
		dependencyTrackClient = faketrack.New()
	} else {
		hookdClient = hookd.New(cfg.Hookd.Endpoint, cfg.Hookd.PSK, errorsCounter, log.WithField("client", "hookd"))
		dependencyTrackClient = dependencytrack.New(
			cfg.DependencyTrack.Endpoint,
			cfg.DependencyTrack.Username,
			cfg.DependencyTrack.Password,
			cfg.DependencyTrack.Frontend,
			log.WithField("client", "dependencytrack"),
		)
	}

	userSyncRuns := usersync.NewRunsHandler(cfg.UserSync.RunsToPersist)
	resourceUsageClient := resourceusage.NewClient(cfg.K8s.AllClusterNames(), db, log)
	resolver := graph.NewResolver(
		hookdClient,
		k8sClient,
		dependencyTrackClient,
		resourceUsageClient,
		db,
		cfg.TenantDomain,
		userSync,
		auditLogger,
		clusters,
		userSyncRuns,
		log,
	)
	graphHandler, err := graph.NewHandler(gengql.Config{
		Resolvers: resolver,
		Directives: gengql.DirectiveRoot{
			Admin: directives.Admin(),
			Auth:  directives.Auth(),
		},
	}, meter, log)
	if err != nil {
		return fmt.Errorf("create graph handler: %w", err)
	}

	// User sync
	go func() {
		if !cfg.UserSync.Enabled {
			log.Infof("user sync is disabled")
			for sync := range userSync {
				// drain channel
				log.Infof("draining user sync request with correlation ID %s", sync)
			}
			return
		}

		defer cancel()

		userSyncer, err := usersync.NewFromConfig(cfg.GoogleManagementProjectID, cfg.TenantDomain, cfg.UserSync.AdminGroupPrefix, db, log, userSyncRuns)
		if err != nil {
			log.WithError(err).Errorf("unable to set up user syncer")
			return
		}

		userSyncTimer := time.NewTimer(1 * time.Second)

		for {
			select {
			case <-ctx.Done():
				return

			case correlationID := <-userSync:
				if userSyncer == nil {
					log.Infof("user sync is disabled")
					break
				}

				log.Debug("starting user synchronization...")
				ctx, cancel := context.WithTimeout(ctx, userSyncTimeout)
				err = userSyncer.Sync(ctx, correlationID)
				cancel()

				if err != nil {
					log.WithError(err).Error("sync users")
				}

				log.Debugf("user sync complete")

			case <-userSyncTimer.C:
				nextUserSync := time.Now().Add(userSyncInterval)
				userSyncTimer.Reset(userSyncInterval)
				log.Debugf("scheduled user sync triggered; next run at %s", nextUserSync)

				correlationID, err := uuid.NewUUID()
				if err != nil {
					log.WithError(err).Errorf("unable to create correlation ID for user sync")
					break
				}

				userSync <- correlationID
			}
		}
	}()

	// k8s informers
	go func() {
		stopCh := ctx.Done()
		for cluster, informer := range k8sClient.Informers() {
			log.WithField("cluster", cluster).Infof("starting informers")
			go informer.PodInformer.Informer().Run(stopCh)
			go informer.AppInformer.Informer().Run(stopCh)
			go informer.NaisjobInformer.Informer().Run(stopCh)
			go informer.JobInformer.Informer().Run(stopCh)
			if informer.TopicInformer != nil {
				go informer.TopicInformer.Informer().Run(stopCh)
			}
		}
	}()

	// resource usage updater
	go func() {
		if !cfg.ResourceUtilizationImportEnabled {
			log.Warningf(`resource utilization import is not enabled. Enable by setting the "RESOURCE_UTILIZATION_IMPORT_ENABLED" environment variable to "true"`)
			return
		}

		for env, informers := range k8sClient.Informers() {
			for !informers.AppInformer.Informer().HasSynced() {
				log.Infof("waiting for app informer in %q to sync", env)
				time.Sleep(2 * time.Second)
			}
		}

		promClients, err := getPrometheusClients(cfg.K8s.AllClusterNames(), cfg.Tenant)
		if err != nil {
			log.WithError(err).Errorf("create prometheus clients")
			return
		}

		resourceUsageUpdater := resourceusage.NewUpdater(k8sClient, promClients, db, log)
		if err != nil {
			log.WithError(err).Errorf("create resource usage updater")
			return
		}

		defer cancel()
		err = runResourceUsageUpdater(ctx, resourceUsageUpdater, log.WithField("task", "resource_updater"))
		if err != nil {
			log.WithError(err).Errorf("error in resource usage updater")
		}
	}()

	// cost updater
	go func() {
		if !cfg.Cost.ImportEnabled {
			log.Warningf(`cost import is not enabled. Enable by setting the "COST_DATA_IMPORT_ENABLED" environment variable to "true".`)
			return
		}

		defer cancel()
		err = runCostUpdater(ctx, db, cfg.Tenant, cfg.Cost.BigQueryProjectID, log.WithField("task", "cost_updater"))
		if err != nil {
			log.WithError(err).Errorf("error in cost updater")
		}
	}()

	authHandler, err := setupAuthHandler(cfg.OAuth, db, log)
	if err != nil {
		return err
	}

	// HTTP server
	go func() {
		defer cancel()
		err = getHttpServer(cfg, db, authHandler, graphHandler).ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Infof("unexpected error from HTTP server")
		}
		log.Infof("HTTP server finished, terminating...")
	}()

	// signal handling
	go func() {
		defer cancel()
		sig := <-signals
		log.Infof("received signal %s, terminating...", sig)
	}()

	log.Infof("HTTP server accepting requests on %q", cfg.ListenAddress)
	<-ctx.Done()
	return ctx.Err()
}

// getHttpServer will return a new HTTP server with the specified configuration
func getHttpServer(cfg *Config, db database.Database, authHandler authn.Handler, graphHandler *handler.Server) *http.Server {
	router := chi.NewRouter()
	router.Handle("/metrics", promhttp.Handler())
	router.Get("/healthz", func(_ http.ResponseWriter, _ *http.Request) {})
	router.Get("/", playground.Handler("GraphQL playground", "/query"))

	dataLoaders := dataloader.NewLoaders(db)
	middlewares := []func(http.Handler) http.Handler{}

	if cfg.WithFakeClients {
		middlewares = append(middlewares, auth.StaticUser(db))
	}

	middlewares = append(middlewares,
		cors.New(
			cors.Options{
				AllowedOrigins:   []string{"https://*", "http://*"},
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
				AllowCredentials: true,
			},
		).Handler,

		middleware.ApiKeyAuthentication(db),
		middleware.Oauth2Authentication(db, authHandler),
		dataloader.Middleware(dataLoaders),
	)
	router.Route("/query", func(r chi.Router) {
		r.Use(middlewares...)
		r.Post("/", graphHandler.ServeHTTP)
	})

	return &http.Server{
		Addr:    cfg.ListenAddress,
		Handler: router,
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

// getMetricMeter will return a new metric meter that uses a Prometheus exporter
func getMetricMeter() (met.Meter, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	return provider.Meter("github.com/nais/api"), nil
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

// loadEnvFile will load a .env file if it exists. This is useful for local development.
func loadEnvFile() (fileLoaded bool, err error) {
	if _, err = os.Stat(".env"); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err = godotenv.Load(".env"); err != nil {
		return false, err
	}

	return true, nil
}

type teamChecker struct {
	db database.Database
}

func (t *teamChecker) TeamExists(ctx context.Context, team slug.Slug) bool {
	_, err := t.db.GetTeamBySlug(ctx, team)
	return err == nil
}

func setupAuthHandler(cfg oAuthConfig, db database.Database, log logrus.FieldLogger) (authn.Handler, error) {
	cf := authn.NewGoogle(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL)
	frontendURL, err := url.Parse(cfg.FrontendURL)
	if err != nil {
		return nil, err
	}
	handler := authn.New(cf, db, *frontendURL, log)
	return handler, nil
}
