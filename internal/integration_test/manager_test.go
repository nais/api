//go:build integration_test
// +build integration_test

package integration_test

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/directives"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/k8s/fake"
	"github.com/nais/api/internal/usersync"
	"github.com/nais/tester/testmanager"
	"github.com/nais/tester/testmanager/runner"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

//go:embed testdata/seeds/*.sql
var seeds embed.FS

var (
	postgresContainer *postgres.PostgresContainer
	connectionString  string
)

func TestMain(m *testing.M) {
	flag.Parse()

	ctx := context.Background()
	container, connStr, err := startPostgresql(ctx)
	if err != nil {
		log.Fatal(err)
	}

	postgresContainer = container
	connectionString = connStr

	code := m.Run()

	if err := postgresContainer.Terminate(ctx); err != nil {
		log.Fatalf("failed to terminate container: %s", err)
	}

	os.Exit(code)
}

func TestRunner(t *testing.T) {
	ctx := context.Background()
	mgr := testmanager.New(t, newManager(t))

	if err := mgr.Run(ctx, os.DirFS("./testdata/tests")); err != nil {
		t.Fatal(err)
	}
}

func clusters() graph.ClusterList {
	return graph.ClusterList{
		"dev":     {GCP: true},
		"staging": {GCP: false},
	}
}

func newManager(t *testing.T) testmanager.CreateRunnerFunc[*Config] {
	return func(ctx context.Context, config *Config, state map[string]any) ([]testmanager.Runner, func(), []testmanager.Option, error) {
		if config == nil {
			config = &Config{}
		}
		ctx, done := context.WithCancel(ctx)
		cleanups := []func(){}

		opts := []testmanager.Option{}

		db, pool, cleanup, err := newDB(ctx, !config.SkipSeed)
		if err != nil {
			done()
			return nil, nil, opts, err
		}
		cleanups = append(cleanups, cleanup)

		log := logrus.New()

		log.Out = io.Discard
		if testing.Verbose() {
			log.Out = os.Stdout
			log.Level = logrus.DebugLevel
		}

		topic := newPubsubRunner()

		runners := []testmanager.Runner{
			newGQLRunner(ctx, t, config, db, topic),
			runner.NewSQLRunner(pool),
			topic,
		}

		return runners, func() {
			for _, cleanup := range cleanups {
				cleanup()
			}
			done()
		}, opts, nil
	}
}

func newGQLRunner(ctx context.Context, t *testing.T, config *Config, db database.Database, topic graph.PubsubTopic) testmanager.Runner {
	log := logrus.New()
	log.Out = io.Discard

	auditLogger := auditlogger.New(db, log)

	k8sPath := filepath.Join("testdata", "tests", testmanager.TestDir(ctx), "k8s")

	k8sClient, err := k8s.New(
		"dev-nais",
		k8s.Config{
			Clusters: clusters().GCPClusters(),
		},
		db,
		log.WithField("client", "k8s"),
		k8s.WithClientsCreator(fake.Clients(os.DirFS(k8sPath))),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create k8s client: %s", err))
	}

	if _, err := os.Stat(k8sPath); err == nil {
		if err := k8sClient.Informers().Start(ctx, log); err != nil {
			panic(fmt.Sprintf("failed to start informers: %s", err))
		}
	}

	resolver := graph.NewResolver(nil, k8sClient, nil, nil, db, "dev-nais.io", nil, auditLogger, clusters(), nil, topic, nil, nil)

	hlog := logrus.New()
	hlog.Out = logrusTestLoggerWriter{t: t}
	srv, err := graph.NewHandler(gengql.Config{
		Resolvers: resolver,
		Directives: gengql.DirectiveRoot{
			Auth:  directives.Auth(),
			Admin: directives.Admin(),
		},
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

		h := loader.Middleware(db)
		h(srv).ServeHTTP(w, r)
	})

	return runner.NewGQLRunner(authProxy)
}

func startPostgresql(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	lg := log.New(io.Discard, "", 0)
	if testing.Verbose() {
		lg = log.New(os.Stderr, "", log.LstdFlags)
	}

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithLogger(lg),
		testcontainers.WithImage("docker.io/postgres:16-alpine"),
		postgres.WithDatabase("example"),
		postgres.WithUsername("example"),
		postgres.WithPassword("example"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2)),
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
	if err := container.Snapshot(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, connStr, nil
}

func newDB(ctx context.Context, seed bool) (database.Database, *pgxpool.Pool, func(), error) {
	logr := logrus.New()
	logr.Out = io.Discard

	pool, err := database.NewPool(ctx, connectionString, logr, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}

	db := database.NewQuerier(pool)

	cleanup := func() {
		pool.Close()
		if err := postgresContainer.Restore(ctx); err != nil {
			log.Fatalf("failed to restore: %s", err)
		}
	}

	if seed {
		err := fs.WalkDir(seeds, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			b, err := fs.ReadFile(seeds, path)
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
		for name, o := range clusters() {
			envs = append(envs, &database.Environment{
				Name: name,
				GCP:  o.GCP,
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

	fmt.Println("### Publishing message ###")
	fmt.Println(mp)

	p.Receive("topic", runner.PubSubMessage{
		Msg:        mp,
		Attributes: attrs,
	})

	return "123", nil
}

func (p *pubsubRunner) String() string {
	return "topic"
}

type logrusTestLoggerWriter struct {
	t *testing.T
}

func (l logrusTestLoggerWriter) Write(p []byte) (n int, err error) {
	l.t.Log(string(p))
	return len(p), nil
}
