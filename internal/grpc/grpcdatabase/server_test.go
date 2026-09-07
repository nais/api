package grpcdatabase_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nais/api/internal/grpc/grpcdatabase"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
)

func TestDatabasesServer_List(t *testing.T) {
	ctx := context.Background()

	teamSlug := "myteam"

	// Sorted by (type, environment, name, database), the expected order is:
	//
	//	CLOUD_SQL        dev-gcp  instance-a  db-a
	//	CLOUD_SQL        dev-gcp  instance-a  db-b
	//	CLOUD_SQL        prod-gcp instance-a  db-a
	//	ZALANDO_POSTGRES dev-gcp  pg-a        app
	//	ZALANDO_POSTGRES dev-gcp  pg-b        app
	//
	// Fixtures live under testdata/ — the fake client harness
	// (internal/kubernetes/fake) sets the object namespace from the parent
	// directory name, so files live under <cluster>/<team>/.
	server := newServer(t, ctx)

	type node struct {
		typ         protoapi.DatabaseType
		environment string
		name        string
		database    string
	}

	nodes := func(resp *protoapi.ListDatabasesResponse) []node {
		ret := make([]node, 0, len(resp.Nodes))
		for _, n := range resp.Nodes {
			ret = append(ret, node{typ: n.Type, environment: n.Environment, name: n.Name, database: n.Database})
			if n.TeamSlug != teamSlug {
				t.Errorf("expected team slug %q, got %q", teamSlug, n.TeamSlug)
			}
		}
		return ret
	}

	t.Run("empty result for unknown team", func(t *testing.T) {
		resp, err := server.List(ctx, &protoapi.ListDatabasesRequest{TeamSlug: "no-such-team"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 0 {
			t.Errorf("expected 0 nodes, got %d", len(resp.Nodes))
		}

		if resp.PageInfo.TotalCount != 0 {
			t.Errorf("expected total count 0, got %d", resp.PageInfo.TotalCount)
		}

		if resp.PageInfo.HasNextPage || resp.PageInfo.HasPreviousPage {
			t.Errorf("expected no next or previous page, got next=%v previous=%v", resp.PageInfo.HasNextPage, resp.PageInfo.HasPreviousPage)
		}
	})

	t.Run("deterministic ordering", func(t *testing.T) {
		expected := []node{
			{typ: protoapi.DatabaseType_CLOUD_SQL, environment: "dev-gcp", name: "instance-a", database: "db-a"},
			{typ: protoapi.DatabaseType_CLOUD_SQL, environment: "dev-gcp", name: "instance-a", database: "db-b"},
			{typ: protoapi.DatabaseType_CLOUD_SQL, environment: "prod-gcp", name: "instance-a", database: "db-a"},
			{typ: protoapi.DatabaseType_ZALANDO_POSTGRES, environment: "dev-gcp", name: "pg-a", database: "app"},
			{typ: protoapi.DatabaseType_ZALANDO_POSTGRES, environment: "dev-gcp", name: "pg-b", database: "app"},
		}

		// Repeated calls must return the same order.
		for range 3 {
			resp, err := server.List(ctx, &protoapi.ListDatabasesRequest{TeamSlug: teamSlug})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.PageInfo.TotalCount != int64(len(expected)) {
				t.Fatalf("expected total count %d, got %d", len(expected), resp.PageInfo.TotalCount)
			}

			got := nodes(resp)
			if len(got) != len(expected) {
				t.Fatalf("expected %d nodes, got %d", len(expected), len(got))
			}

			for i := range expected {
				if got[i] != expected[i] {
					t.Errorf("node %d: expected %+v, got %+v", i, expected[i], got[i])
				}
			}
		}
	})

	t.Run("partial final page", func(t *testing.T) {
		resp, err := server.List(ctx, &protoapi.ListDatabasesRequest{
			TeamSlug: teamSlug,
			Limit:    2,
			Offset:   4,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 1 {
			t.Fatalf("expected 1 node, got %d", len(resp.Nodes))
		}

		if resp.Nodes[0].Name != "pg-b" {
			t.Errorf("expected node %q, got %q", "pg-b", resp.Nodes[0].Name)
		}

		if resp.PageInfo.HasNextPage {
			t.Error("expected no next page")
		}

		if !resp.PageInfo.HasPreviousPage {
			t.Error("expected previous page")
		}
	})

	t.Run("offset beyond length", func(t *testing.T) {
		resp, err := server.List(ctx, &protoapi.ListDatabasesRequest{
			TeamSlug: teamSlug,
			Limit:    10,
			Offset:   100,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 0 {
			t.Errorf("expected 0 nodes, got %d", len(resp.Nodes))
		}

		if resp.PageInfo.TotalCount != 5 {
			t.Errorf("expected total count 5, got %d", resp.PageInfo.TotalCount)
		}

		if resp.PageInfo.HasNextPage {
			t.Error("expected no next page")
		}

		if !resp.PageInfo.HasPreviousPage {
			t.Error("expected previous page")
		}
	})

	negative := map[string]*protoapi.ListDatabasesRequest{
		"negative offset":           {TeamSlug: teamSlug, Limit: 2, Offset: -1},
		"negative limit":            {TeamSlug: teamSlug, Limit: -2, Offset: 0},
		"negative limit and offset": {TeamSlug: teamSlug, Limit: -100, Offset: -100},
	}
	for name, req := range negative {
		t.Run(name, func(t *testing.T) {
			// Must not panic.
			resp, err := server.List(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.PageInfo.TotalCount != 5 {
				t.Errorf("expected total count 5, got %d", resp.PageInfo.TotalCount)
			}
		})
	}
}

func newServer(t *testing.T, ctx context.Context) *grpcdatabase.Server {
	t.Helper()

	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatalf("create scheme: %v", err)
	}

	ccm, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev-gcp", "prod-gcp"}, nil)
	if err != nil {
		t.Fatalf("create cluster config: %v", err)
	}

	log, _ := logrustest.NewNullLogger()
	log.SetLevel(logrus.PanicLevel)

	mgr, err := watcher.NewManager(scheme, ccm, log, watcher.WithClientCreator(fake.Clients(os.DirFS("./testdata"))))
	if err != nil {
		t.Fatalf("create watcher manager: %v", err)
	}
	t.Cleanup(mgr.Stop)

	sqlDatabaseWatcher := sqlinstance.NewDatabaseWatcher(ctx, mgr)
	zalandoPostgresWatcher := postgres.NewZalandoPostgresWatcher(ctx, mgr)

	ctxWait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(ctxWait) {
		t.Fatal("timed out waiting for watcher manager")
	}

	return grpcdatabase.NewServer(
		(*watchers.SqlDatabaseWatcher)(sqlDatabaseWatcher),
		(*watchers.ZalandoPostgresWatcher)(zalandoPostgresWatcher),
	)
}
