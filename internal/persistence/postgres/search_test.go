package postgres

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/sirupsen/logrus/hooks/test"
)

type searchRegistration struct {
	search.Client
	clients map[search.SearchType]search.Searchable
}

func (s *searchRegistration) AddClient(kind search.SearchType, client search.Searchable) {
	s.clients[kind] = client
}

func TestPostgresSearchRegistrationIndexAndConversion(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := test.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(os.DirFS("../../../integration_tests/k8s_resources/postgres_workloads"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	ctx := context.Background()
	postgresWatcher := NewPostgresWatcher(ctx, mgr)
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("Postgres watcher did not synchronize")
	}
	ctx = NewLoaderContext(ctx, nil, postgresWatcher, "", "", "nav", mgr.GetDynamicClients())
	client := &searchRegistration{clients: make(map[search.SearchType]search.Searchable)}
	AddSearchPostgres(client, postgresWatcher)
	searchable, ok := client.clients["POSTGRES"]
	if !ok || len(client.clients) != 1 {
		t.Fatalf("registered search clients = %v, want only POSTGRES", client.clients)
	}
	docs := searchable.ReIndex(ctx)
	if len(docs) != 3 {
		t.Fatalf("indexed %d documents, want 3 logical databases", len(docs))
	}
	names := make(map[string]bool)
	for _, doc := range docs {
		id := ident.FromString(doc.ID)
		if id.Type != "PG" || doc.Kind != "POSTGRES" || doc.Team != "postgres-workload-team" {
			t.Errorf("unexpected indexed document: %+v, ID: %+v", doc, id)
		}
		if want := []string{doc.Team, "dev", doc.Name}; !reflect.DeepEqual(id.Parts(), want) {
			t.Errorf("ID parts = %v, want %v", id.Parts(), want)
		}
		names[doc.Name] = true
		nodes, err := searchable.Convert(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 1 {
			t.Fatalf("converted %d nodes, want 1", len(nodes))
		}
		pg, ok := nodes[0].(*Postgres)
		if !ok {
			t.Fatalf("converted node type = %T, want *Postgres", nodes[0])
		}
		if pg.Name != doc.Name || pg.ID() != id || pg.TeamSlug.String() != doc.Team || pg.EnvironmentName != "dev" {
			t.Errorf("converted Postgres = %+v, document = %+v", pg, doc)
		}
	}
	if want := map[string]bool{"archive": true, "orders": true, "reports": true}; !reflect.DeepEqual(names, want) {
		t.Errorf("indexed names = %v, want %v", names, want)
	}
}
