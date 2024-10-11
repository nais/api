package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/cmd/api"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/usersync"
	"github.com/nais/api/internal/v1/graphv1"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	apiRunner "github.com/nais/api/internal/v1/integration/runner"
	"github.com/nais/api/internal/v1/kubernetes"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	testmanager "github.com/nais/tester/lua"
	"github.com/nais/tester/lua/runner"
	"github.com/nais/tester/lua/spec"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestRunner(ctx context.Context, skipSetup bool) (*testmanager.Manager, error) {
	mgr, err := testmanager.New(newConfig, newManager(ctx, skipSetup), &runner.GQL{}, &runner.SQL{}, &runner.PubSub{}, &apiRunner.K8s{})
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

func clusters() []string {
	return []string{"dev", "staging"}
}

func newManager(ctx context.Context, skipSetup bool) testmanager.SetupFunc {
	if skipSetup {
		return func(_ context.Context, _ string, _ any) (runners []spec.Runner, close func(), err error) {
			return nil, func() {}, nil
		}
	}

	container, connStr, err := startPostgresql(ctx)
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, dir string, configInput any) ([]spec.Runner, func(), error) {
		config, ok := configInput.(*Config)
		if !ok {
			config = &Config{}
		}

		ctx, done := context.WithCancel(ctx)
		cleanups := []func(){}

		scheme, err := kubernetes.NewScheme()
		if err != nil {
			done()
			return nil, nil, fmt.Errorf("failed to create k8s scheme: %w", err)
		}

		db, pool, cleanup, err := newDB(ctx, container, connStr, !config.SkipSeed, filepath.Join(dir, "seeds"))
		if err != nil {
			done()
			return nil, nil, err
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
		gqlRunner, err := newGQLRunner(ctx, config, db, topic, k8sRunner)
		if err != nil {
			done()
			return nil, nil, err
		}

		runners := []spec.Runner{
			gqlRunner,
			runner.NewSQLRunner(pool),
			topic,
			k8sRunner,
		}

		return runners, func() {
			for _, cleanup := range cleanups {
				cleanup()
			}
			done()
		}, nil
	}
}

func newGQLRunner(ctx context.Context, config *Config, db database.Database, topic graphv1.PubsubTopic, k8sRunner *apiRunner.K8s) (spec.Runner, error) {
	log := logrus.New()
	log.Out = io.Discard

	watcherMgr, err := watcher.NewManager(k8sRunner.Scheme, "dev-nais", kubernetes.Config{
		Clusters: clusters(),
	}, log.WithField("subsystem", "k8s_watcher"), watcher.WithClientCreator(k8sRunner.ClientCreator))
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher manager: %w", err)
	}

	graphMiddleware, err := api.ConfigureV1Graph(ctx, true, watcherMgr, db, nil, nil, config.TenantName, clusters(), log)
	if err != nil {
		return nil, fmt.Errorf("failed to configure v1 graph: %w", err)
	}

	resolver := graphv1.NewResolver(topic)

	hlog := logrus.New()
	srv, err := graphv1.NewHandler(gengqlv1.Config{
		Resolvers: resolver,
	}, hlog)
	if err != nil {
		panic(fmt.Sprintf("failed to create graph handler: %s", err))
	}

	authProxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !config.Unauthenticated {
			ctx := r.Context()
			email := "authenticated@example.com"
			if config.Admin {
				email = "admin@example.com"
			}

			usr, err := db.GetUserByEmail(ctx, email)
			if err != nil {
				panic(fmt.Sprintf("User with email %q not found", email))
			}

			roles, err := db.GetUserRoles(ctx, usr.ID)
			if err != nil {
				panic(fmt.Sprintf("Unable to get user roles for user with email %q", email))
			}

			r = r.WithContext(authz.ContextWithActor(ctx, usr, roles))
		}

		graphMiddleware(middleware.RequireAuthenticatedUser()(srv)).ServeHTTP(w, r)
	})

	return runner.NewGQLRunner(authProxy), nil
}

func startPostgresql(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("example"),
		postgres.WithUsername("example"),
		postgres.WithPassword("example"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
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
func newDB(ctx context.Context, container *postgres.PostgresContainer, connStr string, seed bool, seeds string) (database.Database, *pgxpool.Pool, func(), error) {
	logr := logrus.New()
	logr.Out = io.Discard

	pool, err := database.NewPool(ctx, connStr, logr, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}

	db := database.NewQuerier(pool)

	cleanup := func() {
		pool.Close()
		if err := container.Restore(ctx, postgres.WithSnapshotName("migrated")); err != nil {
			log.Fatalf("failed to restore: %s", err)
		}
	}

	if seed {
		seedFs := os.DirFS(seeds)
		err := fs.WalkDir(seedFs, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			b, err := fs.ReadFile(seedFs, path)
			if err != nil {
				return err
			}

			if _, err := pool.Exec(ctx, string(b)); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("failed to seed database: %w", err)
		}

		envs := []*database.Environment{}
		for _, name := range clusters() {
			envs = append(envs, &database.Environment{
				Name: name,
				GCP:  true,
			})
		}
		if err := db.SyncEnvironments(ctx, envs); err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("sync environments: %w", err)
		}

		// Assign default roles to all users
		users, _, err := db.GetUsers(ctx, database.Page{Limit: 1000})
		if err != nil {
			cleanup()
			return nil, nil, nil, fmt.Errorf("get users: %w", err)
		}

		for _, usr := range users {
			for _, roleName := range usersync.DefaultRoleNames {
				err = db.AssignGlobalRoleToUser(ctx, usr.ID, roleName)
				if err != nil {
					cleanup()
					return nil, nil, nil, fmt.Errorf("attach default role %q to user %q: %w", roleName, usr.Email, err)
				}
			}
		}
	}

	return db, pool, cleanup, nil
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
