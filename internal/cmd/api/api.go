package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/joho/godotenv"
	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/bigquery"
	"github.com/nais/api/internal/bucket"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/fixtures"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/directives"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/grpc"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/k8s/fake"
	"github.com/nais/api/internal/kafka"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/opensearch"
	"github.com/nais/api/internal/redis"
	"github.com/nais/api/internal/resourceusage"
	fakeresourceusage "github.com/nais/api/internal/resourceusage/fake"
	"github.com/nais/api/internal/sqlinstance"
	"github.com/nais/api/internal/thirdparty/hookd"
	fakehookd "github.com/nais/api/internal/thirdparty/hookd/fake"
	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/v1/graphv1"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/kubernetes"
	fakev1 "github.com/nais/api/internal/v1/kubernetes/fake"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/vulnerability"
	"github.com/nais/api/internal/vulnerabilities"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
)

const (
	exitCodeSuccess = iota
	exitCodeLoggerError
	exitCodeRunError
	exitCodeConfigError
	exitCodeEnvFileError
)

func Run(ctx context.Context) {
	log := logrus.StandardLogger()

	if err := loadEnvFile(log); err != nil {
		log.WithError(err).Errorf("error loading .env file")
		os.Exit(exitCodeEnvFileError)
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

	ctx, signalStop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer signalStop()

	if cfg.WithFakeClients {
		log.Warn("using fake clients")
	}

	_, promReg, err := newMeterProvider(ctx)
	if err != nil {
		return fmt.Errorf("create metric meter: %w", err)
	}

	log.Info("connecting to database")
	db, closer, err := database.New(ctx, cfg.DatabaseConnectionString, log.WithField("subsystem", "database"))
	if err != nil {
		return fmt.Errorf("setting up database: %w", err)
	}
	defer closer()

	// Sync environments to database
	syncEnvs := []*database.Environment{}
	for name, env := range cfg.K8s.GraphClusterList() {
		syncEnvs = append(syncEnvs, &database.Environment{
			Name: name,
			GCP:  env.GCP,
		})
	}

	if err := db.SyncEnvironments(ctx, syncEnvs); err != nil {
		return fmt.Errorf("sync environments to database: %w", err)
	}

	if err := fixtures.SetupStaticServiceAccounts(ctx, db, cfg.StaticServiceAccounts); err != nil {
		return err
	}

	k8sOpts := []k8s.Opt{}
	if cfg.WithFakeClients {
		k8sOpts = append(k8sOpts, k8s.WithClientsCreator(fake.Clients(os.DirFS("./data/k8s"))))
	}

	k8sClient, err := k8s.New(
		cfg.Tenant,
		cfg.K8s.PkgConfig(),
		db,
		cfg.WithFakeClients,
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

	unleashOpts := []unleash.Opt{}
	if cfg.WithFakeClients {
		unleashOpts = append(unleashOpts, unleash.WithClientsCreator(fake.Clients(os.DirFS("./data/k8s"))))
	}

	unleashMgr, err := unleash.NewManager(
		cfg.Tenant,
		cfg.K8s.PkgConfig().Clusters,
		cfg.Unleash.PkgConfig(),
		log.WithField("client", "unleash"),
		unleashOpts...,
	)
	if err != nil {
		return fmt.Errorf("unable to create unleash manager: %w", err)
	}
	if err := unleashMgr.Start(ctx, log); err != nil {
		return fmt.Errorf("unable to start unleash manager: %w", err)
	}

	auditLogger := auditlogger.New(db, log)

	pubsubClient, err := pubsub.NewClient(ctx, cfg.GoogleManagementProjectID)
	if err != nil {
		return err
	}

	pubsubTopic := pubsubClient.Topic("nais-api")

	var hookdClient graph.HookdClient
	var resourceUsageClient resourceusage.ResourceUsageClient
	if cfg.WithFakeClients {
		hookdClient = fakehookd.New()
		resourceUsageClient = fakeresourceusage.New(db, k8sClient)
	} else {
		hookdClient = hookd.New(cfg.Hookd.Endpoint, cfg.Hookd.PSK, log.WithField("client", "hookd"))
		resourceUsageClient, err = resourceusage.New(cfg.K8s.AllClusterNames(), cfg.Tenant, log)
		if err != nil {
			return fmt.Errorf("create resource usage client: %w", err)
		}
	}

	sqlInstanceClient, err := sqlinstance.NewClient(ctx, db, k8sClient.Informers(), log, sqlinstance.WithFakeClients(cfg.WithFakeClients))
	if err != nil {
		return fmt.Errorf("create sql instance client: %w", err)
	}

	resolver := graph.NewResolver(
		hookdClient,
		k8sClient,
		vulnerabilities.NewManager(cfg.ToVulnerabilitiesConfig(cfg.K8s.AllClusterNames())),
		resourceUsageClient,
		db,
		cfg.Tenant,
		cfg.TenantDomain,
		auditLogger,
		cfg.K8s.GraphClusterList(),
		pubsubTopic,
		log,
		sqlInstanceClient,
		bucket.NewClient(k8sClient.Informers(), log, db),
		redis.NewClient(k8sClient.Informers(), log, db),
		bigquery.NewClient(k8sClient.Informers(), log, db),
		opensearch.NewClient(k8sClient.Informers(), log, db),
		kafka.NewClient(k8sClient.Informers(), log, db),
		unleashMgr,
		audit.NewAuditor(db),
	)

	graphHandler, err := graph.NewHandler(gengql.Config{
		Resolvers: resolver,
		Directives: gengql.DirectiveRoot{
			Admin: directives.Admin(),
			Auth:  directives.Auth(),
		},
	}, log)
	if err != nil {
		return fmt.Errorf("create graph handler: %w", err)
	}

	scheme, err := kubernetes.NewScheme()
	if err != nil {
		return fmt.Errorf("create k8s scheme: %w", err)
	}

	watcherOpts := []watcher.Option{}
	if cfg.WithFakeClients {
		watcherOpts = append(watcherOpts, watcher.WithClientCreator(fakev1.Clients(os.DirFS("./data/k8s"))))
	}

	clusterConfig, err := kubernetes.CreateClusterConfigMap(cfg.Tenant, cfg.K8s.Clusters)
	if err != nil {
		return fmt.Errorf("creating cluster config map: %w", err)
	}

	watcherMgr, err := watcher.NewManager(scheme, clusterConfig, log.WithField("subsystem", "k8s_watcher"), watcherOpts...)
	if err != nil {
		return fmt.Errorf("create k8s watcher manager: %w", err)
	}

	mgmtWatcher, err := watcher.NewManager(scheme, kubernetes.ClusterConfigMap{"management": nil}, log.WithField("subsystem", "k8s_watcher"), watcherOpts...)
	if err != nil {
		return fmt.Errorf("create k8s watcher manager for management: %w", err)
	}

	k8sClientSets, err := kubernetes.NewClientSets(clusterConfig)
	if err != nil {
		return fmt.Errorf("create k8s client sets: %w", err)
	}

	graphv1Handler, err := graphv1.NewHandler(gengqlv1.Config{
		Resolvers: graphv1.NewResolver(
			&graphv1.TopicWrapper{Topic: pubsubTopic},
			graphv1.WithLogger(log),
		),
	}, log)
	if err != nil {
		return fmt.Errorf("create graphv1 handler: %w", err)
	}

	wg, ctx := errgroup.WithContext(ctx)

	wg.Go(func() error {
		return runUsersync(ctx, cfg, db, log)
	})

	// k8s informers
	if err := k8sClient.Informers().Start(ctx, log); err != nil {
		return fmt.Errorf("start k8s informers: %w", err)
	}

	wg.Go(func() error {
		return costUpdater(ctx, cfg, db, log)
	})

	authHandler, err := setupAuthHandler(cfg.OAuth, db, log)
	if err != nil {
		return err
	}

	vulnClient := vulnerability.NewDependencyTrackClient(vulnerability.DependencyTrackConfig{
		Endpoint:    cfg.DependencyTrack.Endpoint,
		Username:    cfg.DependencyTrack.Username,
		Password:    cfg.DependencyTrack.Password,
		FrontendURL: cfg.DependencyTrack.Frontend,
		EnableFakes: cfg.WithFakeClients,
	},
		log.WithField("client", "dependencytrack"),
	)

	// HTTP server
	wg.Go(func() error {
		return runHttpServer(
			ctx,
			cfg.ListenAddress,
			cfg.WithFakeClients,
			cfg.Tenant,
			cfg.K8s.Clusters,
			db,
			k8sClientSets,
			watcherMgr,
			mgmtWatcher,
			sqlInstanceClient.Admin,
			authHandler,
			graphHandler,
			graphv1Handler,
			promReg,
			vulnClient,
			hookdClient,
			log,
		)
	})

	wg.Go(func() error {
		if err := grpc.Run(ctx, cfg.GRPCListenAddress, db, log); err != nil {
			log.WithError(err).Errorf("error in GRPC server")
			return err
		}
		return nil
	})

	<-ctx.Done()
	signalStop()
	log.Infof("shutting down...")

	ch := make(chan error)
	go func() {
		ch <- wg.Wait()
	}()

	select {
	case <-time.After(10 * time.Second):
		log.Warn("timed out waiting for graceful shutdown")
	case err := <-ch:
		return err
	}

	return nil
}

// loadEnvFile will load a .env file if it exists. This is useful for local development.
func loadEnvFile(log logrus.FieldLogger) error {
	if _, err := os.Stat(".env"); errors.Is(err, os.ErrNotExist) {
		log.Infof("no .env file found")
		return nil
	}

	if err := godotenv.Load(".env"); err != nil {
		return err
	}

	log.Infof("loaded .env file")
	return nil
}

func setupAuthHandler(cfg oAuthConfig, db database.Database, log logrus.FieldLogger) (authn.Handler, error) {
	cf, err := authn.NewGoogle(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL)
	if err != nil {
		return nil, err
	}
	handler := authn.New(cf, db, log)
	return handler, nil
}
