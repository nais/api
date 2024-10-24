//go:build integration_test
// +build integration_test

package utilization

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/v1/kubernetes"
	"github.com/nais/api/internal/v1/kubernetes/fake"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload/application"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestFakeQuery(t *testing.T) {
	now := func() prom.Time {
		return prom.TimeFromUnix(100000)
	}

	ctx := context.Background()

	tests := map[string]struct {
		query    string
		args     []any
		expected prom.Vector
	}{
		"appCPURequest": {
			query: appCPURequest,
			args:  []any{"team", "workload"},
			expected: prom.Vector{
				{Metric: prom.Metric{"container": "workload"}, Value: 0.15658213673311283, Timestamp: now()},
			},
		},
		"appCPUUsage": {
			query: appCPUUsage,
			args:  []any{"team", "workload"},
			expected: prom.Vector{
				{Metric: prom.Metric{"container": "workload"}, Value: 0.15658213673311283, Timestamp: now()},
			},
		},
		"appMemoryRequest": {
			query: appMemoryRequest,
			args:  []any{"team", "workload"},
			expected: prom.Vector{
				{Metric: prom.Metric{"container": "workload"}, Value: 105283867, Timestamp: now()},
			},
		},
		"appMemoryUsage": {
			query: appMemoryUsage,
			args:  []any{"team", "workload"},
			expected: prom.Vector{
				{Metric: prom.Metric{"container": "workload"}, Value: 105283867, Timestamp: now()},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewFakeClient([]string{"test", "dev"}, rand.New(rand.NewPCG(2, 2)), now)

			res, err := c.query(ctx, "unused", fmt.Sprintf(test.query, test.args...))
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if !cmp.Equal(test.expected, res) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(test.expected, res))
			}
		})
	}
}

func TestFakeQueryAll(t *testing.T) {
	now := func() prom.Time {
		return prom.TimeFromUnix(100000)
	}

	tests := map[string]struct {
		query    string
		args     []any
		expected map[string]prom.Vector
	}{
		"teamsMemoryRequest": {
			query: teamsMemoryRequest,
			args:  []any{"", ""},
			expected: map[string]prom.Vector{
				"dev":  {&prom.Sample{Metric: prom.Metric{"namespace": "team1"}, Value: 313949612, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"namespace": "team2"}, Value: 910654684, Timestamp: now()}},
				"test": {&prom.Sample{Metric: prom.Metric{"namespace": "team1"}, Value: 750014392, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"namespace": "team2"}, Value: 487676427, Timestamp: now()}},
			},
		},
		"teamsMemoryUsage": {
			query: teamsMemoryUsage,
			args:  []any{"", ""},
			expected: map[string]prom.Vector{
				"dev":  {&prom.Sample{Metric: prom.Metric{"namespace": "team1"}, Value: 313949612, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"namespace": "team2"}, Value: 910654684, Timestamp: now()}},
				"test": {&prom.Sample{Metric: prom.Metric{"namespace": "team1"}, Value: 750014392, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"namespace": "team2"}, Value: 487676427, Timestamp: now()}},
			},
		},
		"teamsCPURequest": {
			query: teamsCPURequest,
			args:  []any{"", ""},
			expected: map[string]prom.Vector{
				"dev":  {&prom.Sample{Metric: prom.Metric{"namespace": "team1"}, Value: 1.6575697128208544, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"namespace": "team2"}, Value: 1.6466714936466849, Timestamp: now()}},
				"test": {&prom.Sample{Metric: prom.Metric{"namespace": "team1"}, Value: 0.6805719573212468, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"namespace": "team2"}, Value: 1.8199158760450043, Timestamp: now()}},
			},
		},
		"teamsCPUUsage": {
			query: teamsCPUUsage,
			args:  []any{"", ""},
			expected: map[string]prom.Vector{
				"dev":  {&prom.Sample{Metric: prom.Metric{"namespace": "team1"}, Value: 1.6575697128208544, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"namespace": "team2"}, Value: 1.6466714936466849, Timestamp: now()}},
				"test": {&prom.Sample{Metric: prom.Metric{"namespace": "team1"}, Value: 0.6805719573212468, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"namespace": "team2"}, Value: 1.8199158760450043, Timestamp: now()}},
			},
		},
		"teamMemoryRequest": {
			query: teamMemoryRequest,
			args:  []any{"team1", ""},
			expected: map[string]prom.Vector{
				"dev":  {&prom.Sample{Metric: prom.Metric{"container": "app-name-dev"}, Value: 313949612, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"container": "app-second-dev"}, Value: 910654684, Timestamp: now()}},
				"test": {&prom.Sample{Metric: prom.Metric{"container": "app-name-test"}, Value: 750014392, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"container": "app-second-test"}, Value: 487676427, Timestamp: now()}},
			},
		},
		"teamMemoryUsage": {
			query: teamMemoryUsage,
			args:  []any{"team2", ""},
			expected: map[string]prom.Vector{
				"dev":  {&prom.Sample{Metric: prom.Metric{"container": "team-app-name-dev"}, Value: 313949612, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"container": "team-app-second-dev"}, Value: 910654684, Timestamp: now()}},
				"test": {&prom.Sample{Metric: prom.Metric{"container": "team-app-name-test"}, Value: 750014392, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"container": "team-app-second-test"}, Value: 487676427, Timestamp: now()}},
			},
		},
		"teamCPURequest": {
			query: teamCPURequest,
			args:  []any{"team2", ""},
			expected: map[string]prom.Vector{
				"dev":  {&prom.Sample{Metric: prom.Metric{"container": "team-app-name-dev"}, Value: 1.6575697128208544, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"container": "team-app-second-dev"}, Value: 1.6466714936466849, Timestamp: now()}},
				"test": {&prom.Sample{Metric: prom.Metric{"container": "team-app-name-test"}, Value: 0.6805719573212468, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"container": "team-app-second-test"}, Value: 1.8199158760450043, Timestamp: now()}},
			},
		},
		"teamCPUUsage": {
			query: teamCPUUsage,
			args:  []any{"team1", ""},
			expected: map[string]prom.Vector{
				"dev":  {&prom.Sample{Metric: prom.Metric{"container": "app-name-dev"}, Value: 1.6575697128208544, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"container": "app-second-dev"}, Value: 1.6466714936466849, Timestamp: now()}},
				"test": {&prom.Sample{Metric: prom.Metric{"container": "app-name-test"}, Value: 0.6805719573212468, Timestamp: now()}, &prom.Sample{Metric: prom.Metric{"container": "app-second-test"}, Value: 1.8199158760450043, Timestamp: now()}},
			},
		},
	}

	ctx := context.Background()
	container, connStr, err := startPostgresql(ctx)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer container.Terminate(ctx)

	pool, cleanup, err := newDB(ctx, container, connStr)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer cleanup()

	// Create teams
	_, err = pool.Exec(ctx, "INSERT INTO teams (slug, slack_channel, purpose) VALUES ('team1', '#asd', 'p'), ('team2', '#asd', 'p')")
	if err != nil {
		t.Fatalf("failed to insert teams: %v", err)
	}

	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatalf("failed to create kubernetes scheme: %v", err)
	}

	tenant := "nav"
	clusters := []string{"test", "dev"}
	ccm, err := kubernetes.CreateClusterConfigMap(tenant, clusters)
	if err != nil {
		t.Fatalf("failed to create cluster config map: %v", err)
	}

	mgr, err := watcher.NewManager(scheme, ccm, logrus.New(), watcher.WithClientCreator(fake.Clients(os.DirFS("./testdata/"))))
	if err != nil {
		t.Fatalf("failed to create watcher manager: %v", err)
	}

	ctx = application.NewLoaderContext(ctx, application.NewWatcher(ctx, mgr), application.NewIngressWatcher(ctx, mgr))
	ctx = team.NewLoaderContext(ctx, pool, nil)

	ctxWait, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	if !mgr.WaitForReady(ctxWait) {
		t.Fatalf("timed out waiting for watcher manager to be ready")
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewFakeClient([]string{"test", "dev"}, rand.New(rand.NewPCG(1, 1)), now)

			res, err := c.queryAll(ctx, fmt.Sprintf(test.query, test.args...))
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if !cmp.Equal(test.expected, res) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(test.expected, res))
			}
		})
	}
}

func TestFakeQueryRange(t *testing.T) {
	start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() prom.Time {
		return prom.TimeFromUnix(start.Unix())
	}

	tests := map[string]struct {
		query    string
		args     []any
		rng      promv1.Range
		expected prom.Matrix
	}{
		"appMemoryUsage": {
			query: appMemoryUsage,
			rng:   promv1.Range{Start: start.Add(-5 * time.Minute), End: start, Step: 15 * time.Second},
			args:  []any{"team1", "workload1"},
			expected: prom.Matrix{
				{
					Metric: prom.Metric{"container": "workload1"},
					Values: []prom.SamplePair{
						{Value: 750014392, Timestamp: 1609458900 * 1000},
						{Value: 487676427, Timestamp: 1609458915 * 1000},
						{Value: 313949612, Timestamp: 1609458930 * 1000},
						{Value: 910654684, Timestamp: 1609458945 * 1000},
						{Value: 967117253, Timestamp: 1609458960 * 1000},
						{Value: 341744470, Timestamp: 1609458975 * 1000},
						{Value: 766083810, Timestamp: 1609458990 * 1000},
						{Value: 145826413, Timestamp: 1609459005 * 1000},
						{Value: 380806771, Timestamp: 1609459020 * 1000},
						{Value: 390512412, Timestamp: 1609459035 * 1000},
						{Value: 574103597, Timestamp: 1609459050 * 1000},
						{Value: 848433386, Timestamp: 1609459065 * 1000},
						{Value: 1038353551, Timestamp: 1609459080 * 1000},
						{Value: 805217662, Timestamp: 1609459095 * 1000},
						{Value: 131562733, Timestamp: 1609459110 * 1000},
						{Value: 995487816, Timestamp: 1609459125 * 1000},
						{Value: 375413312, Timestamp: 1609459140 * 1000},
						{Value: 778198426, Timestamp: 1609459155 * 1000},
						{Value: 631873318, Timestamp: 1609459170 * 1000},
						{Value: 559655994, Timestamp: 1609459185 * 1000},
					},
				},
			},
		},
		"appCPUUsage": {
			query: appCPUUsage,
			rng:   promv1.Range{Start: start.Add(-5 * time.Minute), End: start, Step: 15 * time.Second},
			args:  []any{"team1", "workload1"},
			expected: prom.Matrix{
				{
					Metric: prom.Metric{"container": "workload1"},
					Values: []prom.SamplePair{
						{Value: 0.6805719573212468, Timestamp: 1609458900 * 1000},
						{Value: 1.8199158760450043, Timestamp: 1609458915 * 1000},
						{Value: 1.6575697128208544, Timestamp: 1609458930 * 1000},
						{Value: 1.6466714936466849, Timestamp: 1609458945 * 1000},
						{Value: 0.9564923966105294, Timestamp: 1609458960 * 1000},
						{Value: 1.9056773907302236, Timestamp: 1609458975 * 1000},
						{Value: 1.5797440285315862, Timestamp: 1609458990 * 1000},
						{Value: 1.5704598713365627, Timestamp: 1609459005 * 1000},
						{Value: 1.1737330768177372, Timestamp: 1609459020 * 1000},
						{Value: 0.8441251154108107, Timestamp: 1609459035 * 1000},
						{Value: 0.47002518631235124, Timestamp: 1609459050 * 1000},
						{Value: 0.9919459282255567, Timestamp: 1609459065 * 1000},
						{Value: 0.15924881730487406, Timestamp: 1609459080 * 1000},
						{Value: 1.40272801739988, Timestamp: 1609459095 * 1000},
						{Value: 1.2709689395194872, Timestamp: 1609459110 * 1000},
						{Value: 1.0979349402177991, Timestamp: 1609459125 * 1000},
						{Value: 1.8316119513525706, Timestamp: 1609459140 * 1000},
						{Value: 0.3501486084768204, Timestamp: 1609459155 * 1000},
						{Value: 1.1640926773197378, Timestamp: 1609459170 * 1000},
						{Value: 0.9027727855671075, Timestamp: 1609459185 * 1000},
					},
				},
			},
		},
	}

	ctx := context.Background()

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewFakeClient([]string{"test", "dev"}, rand.New(rand.NewPCG(1, 1)), now)

			res, _, err := c.queryRange(ctx, "test", fmt.Sprintf(test.query, test.args...), test.rng)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if !cmp.Equal(test.expected, res) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(test.expected, res))
			}
		})
	}
}

func startPostgresql(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	lg := log.New(io.Discard, "", 0)
	// if testing.Verbose() {
	// 	lg = log.New(os.Stderr, "", log.LstdFlags)
	// }

	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		testcontainers.WithLogger(lg),
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

func newDB(ctx context.Context, postgresContainer *postgres.PostgresContainer, connectionString string) (*pgxpool.Pool, func(), error) {
	logr := logrus.New()
	logr.Out = io.Discard

	pool, err := database.NewPool(ctx, connectionString, logr, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}

	cleanup := func() {
		pool.Close()
		if err := postgresContainer.Restore(ctx); err != nil {
			log.Fatalf("failed to restore: %s", err)
		}
	}

	return pool, cleanup, nil
}
