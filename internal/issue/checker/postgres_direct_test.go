package checker

import (
	"context"
	"testing"
	"time"

	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/postgres"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestPostgresIssuesWithoutBranchObjects(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logrustest.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "Postgres",
		"metadata": map[string]any{"name": "orders", "namespace": "team", "generation": int64(1)},
		"spec":     map[string]any{"majorVersion": "18"},
		"status": map[string]any{
			"clusterName": "pg-existing-main-12345678", "observedGeneration": int64(1), "reconcilePhase": "Completed",
			"conditions": []any{map[string]any{"type": "cluster.postgresql.cnpg.io/ObservedState", "status": "True", "message": "Cluster is in phase: Cluster is unrecoverable and needs manual intervention"}},
		},
	}}
	client := mgr.GetDynamicClients()["dev"].Resource(nais_io_v1.GroupVersion.WithResource("postgres")).Namespace("team")
	if _, err := client.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	w := postgres.NewPostgresWatcher(ctx, mgr)
	ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
	defer cancelReady()
	if !mgr.WaitForReady(ready) {
		t.Fatal("Postgres watcher did not synchronize")
	}
	check := Postgres{PostgresWatcher: w}
	issues, err := check.Run(ctx)
	if err != nil || len(issues) != 1 {
		t.Fatalf("issues = %v, error = %v", issues, err)
	}
	if issues[0].ResourceName != nais_io_v1.PostgresPhysicalName("orders") || issues[0].Severity != issue.SeverityCritical {
		t.Errorf("direct issue lost its compatible identity/severity: %+v", issues[0])
	}
	if err := client.Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		issues, err := check.Run(ctx)
		return len(issues) == 0, err
	}); err != nil {
		t.Fatal("deleted Postgres retained its issue:", err)
	}
}
