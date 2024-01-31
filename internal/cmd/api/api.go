package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/fixtures"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/directives"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/grpc"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/k8s/fake"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/resourceusage"
	"github.com/nais/api/internal/thirdparty/dependencytrack"
	faketrack "github.com/nais/api/internal/thirdparty/dependencytrack/fake"
	"github.com/nais/api/internal/thirdparty/hookd"
	fakehookd "github.com/nais/api/internal/thirdparty/hookd/fake"
	"github.com/nais/api/internal/usersync"
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

const (
	costUpdateSchedule     = time.Hour
	resourceUpdateSchedule = time.Hour
	userSyncInterval       = time.Minute * 15
	userSyncTimeout        = time.Second * 30
)

func Run(ctx context.Context) {
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

	// TODO: Implement new first run logic
	// if err := firstRun(ctx, db, cfg.FirstRunEnableReconcilers, log); err != nil {
	// 	return err
	// }

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

	auditLogger := auditlogger.New(db, logger.ComponentNameGraphqlApi, log)
	userSync := make(chan uuid.UUID, 1)

	pubsubClient, err := pubsub.NewClient(ctx, cfg.GoogleManagementProjectID)
	if err != nil {
		return err
	}

	pubsubTopic := pubsubClient.Topic("nais-api")

	var hookdClient graph.HookdClient
	var dependencyTrackClient graph.DependencytrackClient
	if cfg.WithFakeClients {
		hookdClient = fakehookd.New()
		dependencyTrackClient = faketrack.New()
	} else {
		hookdClient = hookd.New(cfg.Hookd.Endpoint, cfg.Hookd.PSK, log.WithField("client", "hookd"))
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
		cfg.K8s.GraphClusterList(),
		userSyncRuns,
		pubsubTopic,
		log,
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

	wg, ctx := errgroup.WithContext(ctx)

	wg.Go(func() error {
		return runUserSync(ctx, cfg, db, log, userSync, userSyncRuns)
	})

	// k8s informers
	if err := k8sClient.Informers().Start(ctx, log); err != nil {
		return fmt.Errorf("start k8s informers: %w", err)
	}

	wg.Go(func() error {
		return resourceUsageUpdater(ctx, cfg, db, k8sClient, log)
	})
	wg.Go(func() error {
		return costUpdater(ctx, cfg, db, log)
	})

	authHandler, err := setupAuthHandler(cfg.OAuth, db, log)
	if err != nil {
		return err
	}

	// HTTP server
	wg.Go(func() error {
		return runHttpServer(ctx, cfg.ListenAddress, cfg.WithFakeClients, db, authHandler, graphHandler, promReg, log)
	})

	wg.Go(func() error {
		if err := grpc.Run(ctx, cfg.GRPCListenAddress, db, auditLogger, log); err != nil {
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

// func firstRun(ctx context.Context, db database.Database, enableReconcilers []string, log logrus.FieldLogger) error {
// 	firstRun, err := db.IsFirstRun(ctx)
// 	if err != nil {
// 		return err
// 	}
// 	if firstRun {
// 		log.Infof("first run detected ")
// 		firstRunLogger := log.WithField("system", "first-run")
// 		if err := fixtures.SetupDefaultReconcilers(ctx, firstRunLogger, enableReconcilers, db); err != nil {
// 			return err
// 		}

// 		if err := db.FirstRunComplete(ctx); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

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

func setupAuthHandler(cfg oAuthConfig, db database.Database, log logrus.FieldLogger) (authn.Handler, error) {
	cf := authn.NewGoogle(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL)
	frontendURL, err := url.Parse(cfg.FrontendURL)
	if err != nil {
		return nil, err
	}
	handler := authn.New(cf, db, *frontendURL, log)
	return handler, nil
}
