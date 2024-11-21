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
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/grpc"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/leaderelection"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/thirdparty/hookd"
	fakehookd "github.com/nais/api/internal/thirdparty/hookd/fake"
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
	pool, err := database.New(ctx, cfg.DatabaseConnectionString, log.WithField("subsystem", "database"))
	if err != nil {
		return fmt.Errorf("setting up database: %w", err)
	}
	defer pool.Close()

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
	if cfg.WithFakeClients {
		watcherOpts = append(watcherOpts, watcher.WithClientCreator(fake.Clients(os.DirFS("./data/k8s"))))
	}

	clusterConfig, err := kubernetes.CreateClusterConfigMap(cfg.Tenant, cfg.K8s.Clusters, cfg.K8s.StaticClusters)
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

	pubsubClient, err := pubsub.NewClient(ctx, cfg.GoogleManagementProjectID)
	if err != nil {
		return err
	}
	pubsubTopic := pubsubClient.Topic("nais-api")

	graphHandler, err := graph.NewHandler(gengql.Config{
		Resolvers: graph.NewResolver(
			&graph.TopicWrapper{Topic: pubsubTopic},
			graph.WithLogger(log),
		),
	}, log)
	if err != nil {
		return fmt.Errorf("create graph handler: %w", err)
	}

	authHandler, err := setupAuthHandler(cfg.OAuth, log)
	if err != nil {
		return err
	}

	vulnClient := vulnerability.NewDependencyTrackClient(
		vulnerability.DependencyTrackConfig{
			Endpoint:    cfg.DependencyTrack.Endpoint,
			Username:    cfg.DependencyTrack.Username,
			Password:    cfg.DependencyTrack.Password,
			FrontendURL: cfg.DependencyTrack.Frontend,
			EnableFakes: cfg.WithFakeClients,
		},
		log.WithField("client", "dependencytrack"),
	)

	var hookdClient hookd.Client
	var mgmtK8sClient k8s.Interface
	if cfg.WithFakeClients {
		hookdClient = fakehookd.New()
		mgmtK8sClient = k8sfake.NewSimpleClientset()
	} else {
		hookdClient = hookd.New(cfg.Hookd.Endpoint, cfg.Hookd.PSK, log.WithField("client", "hookd"))
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("creating in-cluster config: %w", err)
		}
		mgmtK8sClient, err = k8s.NewForConfig(cfg)
		if err != nil {
			return fmt.Errorf("creating k8s client: %w", err)
		}
	}

	if err := leaderelection.Start(ctx, mgmtK8sClient, cfg.LeaseName, cfg.LeaseNamespace, log); err != nil {
		return fmt.Errorf("starting leader election: %w", err)
	}

	wg, ctx := errgroup.WithContext(ctx)

	// HTTP server
	wg.Go(func() error {
		return runHttpServer(
			ctx,
			cfg.ListenAddress,
			cfg.WithFakeClients,
			cfg.Tenant,
			cfg.K8s.AllClusterNames(),
			pool,
			clusterConfig,
			watcherMgr,
			mgmtWatcher,
			authHandler,
			graphHandler,
			promReg,
			vulnClient,
			hookdClient,
			log,
		)
	})

	wg.Go(func() error {
		if err := grpc.Run(ctx, cfg.GRPCListenAddress, pool, log); err != nil {
			log.WithError(err).Errorf("error in GRPC server")
			return err
		}
		return nil
	})

	wg.Go(func() error {
		return runUsersync(ctx, pool, cfg, log)
	})

	wg.Go(func() error {
		return costUpdater(ctx, pool, cfg, log)
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

func setupAuthHandler(cfg oAuthConfig, log logrus.FieldLogger) (authn.Handler, error) {
	cf, err := authn.NewGoogle(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL)
	if err != nil {
		return nil, err
	}
	return authn.New(cf, log), nil
}
