package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestLegacyPostgresGrantValidatesTheOldCluster(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := test.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(os.DirFS("../../../integration_tests/k8s_resources/grant_zalando_postgres_access"))))
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
	ctx = NewLoaderContext(ctx, postgresWatcher, "", "", "nav")
	input := GrantPostgresAccessInput{
		ClusterName: "foobar", TeamSlug: slug.Slug("someteamname"), EnvironmentName: "dev",
		Grantee: "someone@example.com", Duration: "30m",
	}
	if err := input.Validate(ctx); err != nil {
		t.Fatalf("existing legacy cluster rejected: %v", err)
	}
	input.ClusterName = "missing"
	if err := input.Validate(ctx); err == nil {
		t.Fatal("missing legacy cluster accepted")
	}
}
