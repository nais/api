package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	aiven_service "github.com/aiven/go-client-codegen"
	"github.com/joho/godotenv"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/notify"
	"github.com/nais/api/internal/deployment"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/grpc"
	"github.com/nais/api/internal/issue/checker"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/event"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/leaderelection"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/loki"
	"github.com/nais/api/internal/persistence/sqlinstance"
	restserver "github.com/nais/api/internal/rest"
	"github.com/nais/api/internal/servicemaintenance"
	"github.com/nais/api/internal/thirdparty/aiven"
	"github.com/nais/api/internal/thirdparty/hookd"
	fakehookd "github.com/nais/api/internal/thirdparty/hookd/fake"
	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/vulnerability"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	k8s "k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
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

	if _, ok := os.LookupEnv("WITH_FAKE_CLIENTS"); ok {
		log.Errorf("WITH_FAKE_CLIENTS should no longer be used. Update your .env file or environment variables.")
		log.Errorf("See .env.example for new environment variables.")
		os.Exit(1)
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

	cfg.Fakes.Inform(appLogger)

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

	_, promReg, err := newMeterProvider(ctx)
	if err != nil {
		return fmt.Errorf("create metric meter: %w", err)
	}

	log.Info("connecting to database")
	dbSettings := []database.OptFunc{}
	if cfg.WithSlowQueryLogger {
		dbSettings = append(dbSettings, database.WithSlowQueryLogger(10*time.Millisecond))
	}
	if cfg.CloudSQLInstance != "" {
		dbSettings = append(dbSettings, database.WithCloudSQLInstance(cfg.CloudSQLInstance))
	}

	pool, err := database.New(ctx, cfg.DatabaseConnectionString, log.WithField("subsystem", "database"), dbSettings...)
	if err != nil {
		return fmt.Errorf("setting up database: %w", err)
	}
	defer pool.Close()

	environmentmapper.SetMapping(cfg.ReplaceEnvironmentNames)

	if err := syncEnvironments(ctx, pool, cfg.K8s.ClusterList()); err != nil {
		return err
	}

	if err := setupStaticServiceAccounts(ctx, pool, cfg.StaticServiceAccounts); err != nil {
		return err
	}

	scheme, err := kubernetes.NewScheme()
	if err != nil {
		return fmt.Errorf("create k8s scheme: %w", err)
	}

	watcherOpts := []watcher.Option{}
	if cfg.Fakes.WithFakeKubernetes {
		watcherOpts = append(watcherOpts, watcher.WithClientCreator(fake.Clients(os.DirFS("./data/k8s"))))
	}

	mgmtWatcherOpts := []watcher.Option{}
	if cfg.Fakes.WithFakeKubernetes || cfg.Fakes.WithFakeManagementKubernetes {
		mgmtWatcherOpts = append(mgmtWatcherOpts, watcher.WithClientCreator(fake.Clients(os.DirFS("./data/k8s"))))
	}

	clusterConfig, err := kubernetes.CreateClusterConfigMap(cfg.Tenant, cfg.K8s.Clusters, cfg.K8s.StaticClusters)
	if err != nil {
		return fmt.Errorf("creating cluster config map: %w", err)
	}

	watcherMgr, err := watcher.NewManager(scheme, clusterConfig, log.WithField("subsystem", "k8s_watcher"), watcherOpts...)
	if err != nil {
		return fmt.Errorf("create k8s watcher manager: %w", err)
	}
	defer watcherMgr.Stop()

	mgmtWatcher, err := watcher.NewManager(scheme, kubernetes.ClusterConfigMap{"management": nil}, log.WithField("subsystem", "k8s_watcher"), mgmtWatcherOpts...)
	if err != nil {
		return fmt.Errorf("create k8s watcher manager for management: %w", err)
	}
	defer mgmtWatcher.Stop()

	watchers := watchers.SetupWatchers(ctx, watcherMgr, mgmtWatcher)

	pubsubClient, err := pubsub.NewClient(ctx, cfg.GoogleManagementProjectID)
	if err != nil {
		return err
	}
	pubsubTopic := pubsubClient.Topic(cfg.PubSub.APITopic)

	graphHandler, err := graph.NewHandler(gengql.Config{
		Resolvers: graph.NewResolver(
			&graph.TopicWrapper{Topic: pubsubTopic},
			graph.WithLogger(log),
		),
		Complexity: gengql.NewComplexityRoot(),
	}, log.WithField("subsystem", "graph"))
	if err != nil {
		return fmt.Errorf("create graph handler: %w", err)
	}

	authHandler, err := setupAuthHandler(ctx, cfg.OAuth, log.WithField("subsystem", "auth"))
	if err != nil {
		return err
	}

	var aivenClient aiven.AivenClient
	if cfg.Fakes.WithFakeAivenClient {
		aivenClient = aiven.NewFakeAivenClient()
	} else {
		pureClient, err := aiven_service.NewClient(
			aiven_service.TokenOpt(cfg.Aiven.Token),
			aiven_service.UserAgentOpt("nais-api"),
		)
		if err != nil {
			return err
		}
		aivenClient = aiven.NewClient(pureClient)
	}

	serviceMaintenanceManager, err := servicemaintenance.NewManager(ctx, aivenClient, log.WithField("subsystem", "maintenance"))
	if err != nil {
		return err
	}

	vulnMgr, err := vulnerability.NewManager(
		ctx,
		cfg.VulnerabilitiesAPI.Endpoint,
		cfg.VulnerabilitiesAPI.ServiceAccount,
		log.WithField("subsystem", "vulnerability"),
	)
	if err != nil {
		return err
	}

	var mgmtK8sClient k8s.Interface
	if cfg.Fakes.WithFakeKubernetes || cfg.Fakes.WithFakeManagementKubernetes {
		mgmtK8sClient = k8sfake.NewSimpleClientset()
	} else {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("creating in-cluster config: %w", err)
		}
		mgmtK8sClient, err = k8s.NewForConfig(cfg)
		if err != nil {
			return fmt.Errorf("creating k8s client: %w", err)
		}
	}

	var hookdClient hookd.Client
	if cfg.Fakes.WithFakeHookd {
		hookdClient = fakehookd.New()
	} else {
		hookdClient = hookd.New(cfg.Hookd.Endpoint, cfg.Hookd.PSK, log.WithField("client", "hookd"))
	}

	if err := leaderelection.Start(ctx, mgmtK8sClient, cfg.LeaseName, cfg.LeaseNamespace, log.WithField("subsystem", "leaderelection")); err != nil {
		return fmt.Errorf("starting leader election: %w", err)
	}

	wg, ctx := errgroup.WithContext(ctx)

	// Notifier to use only one connection to the database for LISTEN/NOTIFY pattern
	notifier := notify.New(pool, log.WithField("subsystem", "notifier"))
	go notifier.Run(ctx)

	if !cfg.Fakes.WithFakeKubernetes {
		k8sClients, err := kubernetes.NewClientSets(clusterConfig)
		if err != nil {
			return fmt.Errorf("creating k8s clients: %w", err)
		}

		log.WithField("envs", len(k8sClients)).Info("Start event watcher")
		eventWatcher, err := event.NewWatcher(pool, pubsubClient.Subscription(cfg.PubSub.EventsSubscription), k8sClients, watcherMgr.ResourceMappers(), log.WithField("subsystem", "event_watcher"))
		if err != nil {
			return fmt.Errorf("creating event watcher: %w", err)
		}
		go eventWatcher.Run(ctx)
	} else {
		log.Info("Start fake event watcher")
		eventWatcher, err := event.NewWatcher(pool, pubsubClient.Subscription(cfg.PubSub.EventsSubscription), nil, watcherMgr.ResourceMappers(), log.WithField("subsystem", "event_watcher"))
		if err != nil {
			return fmt.Errorf("creating event watcher: %w", err)
		}
		go eventWatcher.Run(ctx)
	}

	var jwtMiddleware func(next http.Handler) http.Handler
	if !cfg.JWT.SkipMiddleware {
		jwtMiddleware, err = middleware.JWTAuthentication(ctx, cfg.JWT.Issuer, cfg.JWT.Audience, cfg.Zitadel.OrganizationID, log.WithField("subsystem", "jwt"))
		if err != nil {
			return fmt.Errorf("failed to create JWT authentication middleware: %w", err)
		}
	}

	var lokiClientOpts []loki.OptionFunc
	if addr, ok := os.LookupEnv("LOGGING_LOKI_ADDRESS"); ok {
		lokiClientOpts = append(lokiClientOpts, loki.WithLocalLoki(addr))
	}

	lokiClient, err := loki.NewClient(cfg.K8s.AllClusterNames(), cfg.Tenant, log.WithField("subsystem", "loki_client"), lokiClientOpts...)
	if err != nil {
		return fmt.Errorf("create loki client: %w", err)
	}

	// HTTP server
	wg.Go(func() error {
		return runHTTPServer(
			ctx,
			cfg.Fakes,
			cfg.ListenAddress,
			cfg.Tenant,
			cfg.K8s.AllClusterNames(),
			pool,
			clusterConfig,
			watchers,
			watcherMgr,
			jwtMiddleware,
			authHandler,
			graphHandler,
			serviceMaintenanceManager,
			aivenClient,
			cfg.Aiven.Projects,
			vulnMgr,
			hookdClient,
			cfg.Unleash.BifrostAPIURL,
			cfg.K8s.AllClusterNames(),
			cfg.Logging.DefaultLogDestinations(),
			notifier,
			lokiClient,
			cfg.AuditLog.ProjectID,
			cfg.AuditLog.Location,
			log.WithField("subsystem", "http"),
		)
	})
	wg.Go(func() error {
		return runInternalHTTPServer(
			ctx,
			cfg.InternalListenAddress,
			promReg,
			log.WithField("subsystem", "internal_http"),
		)
	})

	wg.Go(func() error {
		return restserver.Run(ctx, cfg.RestListenAddress, pool, cfg.RestPreSharedKey, log.WithField("subsystem", "rest"))
	})

	wg.Go(func() error {
		if err := grpc.Run(ctx, cfg.GRPCListenAddress, pool, log.WithField("subsystem", "grpc")); err != nil {
			log.WithError(err).Errorf("error in GRPC server")
			return err
		}
		return nil
	})

	wg.Go(func() error {
		return runUsersync(ctx, pool, cfg, log.WithField("subsystem", "usersync"))
	})

	wg.Go(func() error {
		return costUpdater(ctx, pool, cfg, log.WithField("subsystem", "cost_updater"))
	})

	wg.Go(func() error {
		deployment.RunCleaner(ctx, pool, log.WithField("subsystem", "deployment_cleaner"))
		return nil
	})

	wg.Go(func() error {
		activitylog.RunRefresher(ctx, pool, log.WithField("subsystem", "activitylog_refresher"))
		return nil
	})

	sqlAdminService, err := sqlinstance.NewClient(ctx, log, sqlinstance.WithFakeClients(cfg.Fakes.WithFakeCloudSQL), sqlinstance.WithInstanceWatcher(watchers.SqlInstanceWatcher))
	if err != nil {
		return fmt.Errorf("create SQL Admin service: %w", err)
	}

	// Create Bifrost client for Unleash issue checker
	var bifrostClient unleash.BifrostClient
	if cfg.Unleash.BifrostAPIURL == unleash.FakeBifrostURL {
		bifrostClient = unleash.NewFakeBifrostClient(watchers.UnleashWatcher)
	} else {
		bifrostClient = unleash.NewBifrostClient(cfg.Unleash.BifrostAPIURL, log.WithField("subsystem", "bifrost_client"))
	}

	issueChecker, err := checker.New(
		checker.Config{
			AivenClient:    aivenClient,
			CloudSQLClient: sqlAdminService,
			V13sClient:     vulnMgr.Client,
			Tenant:         cfg.Tenant,
			Clusters:       cfg.K8s.AllClusterNames(),
			BifrostClient:  bifrostClient,
		},
		pool,
		watchers,
		cfg.VulnerabilitiesAPI.Endpoint == vulnerability.FakeVulnerabilitiesAPIURL,
		log.WithField("subsystem", "issue_checker"),
	)
	if err != nil {
		log.WithError(err).Error("setting up issue checker")
		return err
	}

	wg.Go(func() error {
		err = issueChecker.RunChecks(ctx)
		if err != nil {
			log.WithError(err).Error("running issue checks")
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

func setupAuthHandler(ctx context.Context, cfg oAuthConfig, log logrus.FieldLogger) (authn.Handler, error) {
	cf, err := authn.NewOIDC(ctx, cfg.Issuer, cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL, cfg.AdditionalScopes)
	if err != nil {
		return nil, err
	}
	return authn.New(cf, log), nil
}
