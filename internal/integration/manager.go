package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/cmd/api"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/notify"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/gengql"
	apiRunner "github.com/nais/api/internal/integration/runner"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	fakeHookd "github.com/nais/api/internal/thirdparty/hookd/fake"
	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/vulnerability"
	"github.com/nais/api/internal/workload/logging"
	testmanager "github.com/nais/tester/lua"
	"github.com/nais/tester/lua/runner"
	"github.com/nais/tester/lua/spec"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ctxKey int

const databaseKey ctxKey = iota

func TestRunner(ctx context.Context, skipSetup bool) (*testmanager.Manager, error) {
	mgr, err := testmanager.New(newConfig, newManager(ctx, skipSetup), &runner.GQL{}, &runner.SQL{}, &runner.PubSub{}, &apiRunner.K8s{})
	if err != nil {
		return nil, err
	}

	// mgr.AddHelper(createTeam())
	mgr.AddTypemetatable(teamMetatable())
	mgr.AddTypemetatable(userMetatable())

	return mgr, nil
}

func clusters() []string {
	return []string{"dev", "staging", "dev-fss", "dev-gcp"}
}

func newManager(ctx context.Context, skipSetup bool) testmanager.SetupFunc {
	if skipSetup {
		return func(ctx context.Context, _ string, _ any) (retCtx context.Context, runners []spec.Runner, close func(), err error) {
			return ctx, nil, func() {}, nil
		}
	}

	container, connStr, err := startPostgresql(ctx)
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, dir string, configInput any) (context.Context, []spec.Runner, func(), error) {
		config, ok := configInput.(*Config)
		if !ok {
			config = &Config{}
		}

		ctx, done := context.WithCancel(ctx)
		cleanups := []func(){}

		scheme, err := kubernetes.NewScheme()
		if err != nil {
			done()
			return ctx, nil, nil, fmt.Errorf("failed to create k8s scheme: %w", err)
		}

		pool, cleanup, err := newDB(ctx, container, connStr)
		if err != nil {
			done()
			return ctx, nil, nil, err
		}
		cleanups = append(cleanups, cleanup)

		log := logrus.New()
		log.Out = io.Discard

		if testing.Verbose() {
			log.Out = os.Stdout
			log.Level = logrus.DebugLevel
		}

		k8sRunner := apiRunner.NewK8sRunner(scheme, dir, clusters())
		topic := newPubsubRunner()
		gqlRunner, gqlCleanup, err := newGQLRunner(ctx, config, pool, topic, k8sRunner)
		if err != nil {
			done()
			return ctx, nil, nil, err
		}

		cleanups = append([]func(){gqlCleanup}, cleanups...)

		runners := []spec.Runner{
			gqlRunner,
			runner.NewSQLRunner(pool),
			topic,
			k8sRunner,
		}

		ctx = context.WithValue(ctx, databaseKey, pool)
		return ctx, runners, func() {
			for _, cleanup := range cleanups {
				cleanup()
			}
			done()
		}, nil
	}
}

func newGQLRunner(ctx context.Context, config *Config, pool *pgxpool.Pool, topic graph.PubsubTopic, k8sRunner *apiRunner.K8s) (spec.Runner, func(), error) {
	log := logrus.New()
	log.Out = io.Discard

	clusterConfig, err := kubernetes.CreateClusterConfigMap("dev-nais", clusters(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating cluster config map: %w", err)
	}

	watcherMgr, err := watcher.NewManager(k8sRunner.Scheme, clusterConfig, log.WithField("subsystem", "k8s_watcher"), watcher.WithClientCreator(k8sRunner.ClientCreator))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create watcher manager: %w", err)
	}

	managementWatcherMgr, err := watcher.NewManager(
		k8sRunner.Scheme,
		kubernetes.ClusterConfigMap{"management": nil},
		log.WithField("subsystem", "mgmt_k8s_watcher"),
		watcher.WithClientCreator(k8sRunner.ClientCreator),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create management watcher manager: %w", err)
	}

	vulnerabilityClient := vulnerability.NewDependencyTrackClient(vulnerability.DependencyTrackConfig{EnableFakes: true}, log)

	notifierCtx, notifyCancel := context.WithCancel(ctx)
	notifier := notify.New(pool, log, notify.WithRetries(0))
	go notifier.Run(notifierCtx)

	graphMiddleware, err := api.ConfigureGraph(
		ctx,
		true,
		watcherMgr,
		managementWatcherMgr,
		pool,
		clusterConfig,
		vulnerabilityClient,
		config.TenantName,
		clusters(),
		fakeHookd.New(),
		unleash.FakeBifrostURL,
		[]logging.SupportedLogDestination{logging.Loki},
		notifier,
		log,
	)
	if err != nil {
		notifyCancel()
		return nil, nil, fmt.Errorf("failed to configure graph: %w", err)
	}

	resolver := graph.NewResolver(topic)

	hlog := logrus.New()
	srv, err := graph.NewHandler(gengql.Config{
		Resolvers: resolver,
	}, hlog)
	if err != nil {
		panic(fmt.Sprintf("failed to create graph handler: %s", err))
	}

	authProxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		email := ""

		if xemail := r.Header.Get("x-user-email"); xemail != "" {
			email = xemail
		}

		if email != "" {
			usr, err := user.GetByEmail(ctx, email)
			if err != nil {
				panic(fmt.Sprintf("User with email %q not found", email))
			}

			roles, err := authz.ForUser(ctx, usr.UUID)
			if err != nil {
				panic(fmt.Sprintf("Unable to get user roles for user with email: %q, %s", email, err))
			}

			r = r.WithContext(authz.ContextWithActor(ctx, usr, roles))
		}

		middleware.ApiKeyAuthentication()(middleware.RequireAuthenticatedUser()(srv)).ServeHTTP(w, r)
	})

	return runner.NewGQLRunner(graphMiddleware(authProxy)), notifyCancel, nil
}

func startPostgresql(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("example"),
		postgres.WithUsername("example"),
		postgres.WithPassword("example"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	logr := logrus.New()
	logr.Out = io.Discard
	pool, err := database.NewPool(ctx, connStr, logr, true) // Migrate database before snapshotting
	if err != nil {
		return nil, "", fmt.Errorf("failed to create pool: %w", err)
	}
	pool.Close()

	if err := container.Snapshot(ctx, postgres.WithSnapshotName("migrated")); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, connStr, nil
}

// func newDB(ctx context.Context, seed bool, connectionString string) (database.Database, *pgxpool.Pool, func(), error) {
func newDB(ctx context.Context, container *postgres.PostgresContainer, connStr string) (*pgxpool.Pool, func(), error) {
	logr := logrus.New()
	logr.Out = io.Discard

	pool, err := database.NewPool(ctx, connStr, logr, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}

	cleanup := func() {
		pool.Close()
		if err := container.Restore(ctx, postgres.WithSnapshotName("migrated")); err != nil {
			log.Fatalf("failed to restore: %s", err)
		}
	}

	ctx = database.NewLoaderContext(ctx, pool)
	ctx = environment.NewLoaderContext(ctx, pool)
	ctx = user.NewLoaderContext(ctx, pool)
	ctx = authz.NewLoaderContext(ctx, pool)

	c := clusters()
	envs := make([]*environment.Environment, len(c))
	for i, name := range c {
		envs[i] = &environment.Environment{
			Name: name,
			GCP:  true,
		}
	}
	if err := environment.SyncEnvironments(ctx, envs); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("sync environments: %w", err)
	}

	return pool, cleanup, nil
}

type pubsubRunner struct {
	*runner.PubSub
}

func newPubsubRunner() *pubsubRunner {
	ret := &pubsubRunner{}
	ret.PubSub = runner.NewPubSub(nil)
	return ret
}

func (p *pubsubRunner) Publish(ctx context.Context, msg protoreflect.ProtoMessage, attrs map[string]string) (string, error) {
	b, err := protojson.Marshal(msg)
	if err != nil {
		return "", err
	}

	mp := make(map[string]any)
	if err := json.Unmarshal(b, &mp); err != nil {
		return "", err
	}

	p.Receive("topic", runner.PubSubMessage{
		Msg:        mp,
		Attributes: attrs,
	})

	return "123", nil
}

func (p *pubsubRunner) String() string {
	return "topic"
}
