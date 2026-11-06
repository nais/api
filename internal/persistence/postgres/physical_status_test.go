package postgres

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
)

func physicalPostgresObject(name string, status map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "Postgres",
		"metadata": map[string]any{"name": name, "namespace": "team", "generation": int64(2), "creationTimestamp": "2026-01-02T03:04:05Z"},
		"spec":     map[string]any{"majorVersion": "18"},
	}}
	if status != nil {
		obj.Object["status"] = status
	}
	return obj
}

func healthyPhysicalStatus(cluster string) map[string]any {
	return map[string]any{
		"clusterName": cluster, "observedGeneration": int64(2), "reconcilePhase": "Completed",
		"configuration": map[string]any{"parameters": map[string]any{"work_mem": "32MB"}},
		"conditions": []any{map[string]any{
			"type": "cluster.postgresql.cnpg.io/ObservedState", "status": "True",
			"message": "Cluster is in phase: Cluster in healthy state",
		}},
	}
}

func TestDirectPostgresPhysicalStatus(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(map[string]any)
		want   PostgresState
	}{
		{name: "healthy", want: PostgresBranchStateAvailable},
		{name: "old observed generation", change: func(s map[string]any) { s["observedGeneration"] = int64(1) }, want: PostgresBranchStateProgressing},
		{name: "not completed", change: func(s map[string]any) { s["reconcilePhase"] = "Preparing" }, want: PostgresBranchStateProgressing},
		{name: "no physical condition", change: func(s map[string]any) { delete(s, "conditions") }, want: PostgresBranchStateProgressing},
		{name: "degraded", change: func(s map[string]any) {
			s["conditions"].([]any)[0].(map[string]any)["message"] = "Cluster is in phase: Cluster is unrecoverable and needs manual intervention"
		}, want: PostgresBranchStateDegraded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			status := healthyPhysicalStatus("pg-existing-main-12345678")
			if tt.change != nil {
				tt.change(status)
			}
			pg, err := toPostgres(physicalPostgresObject("orders", status), "dev")
			if err != nil {
				t.Fatal(err)
			}
			// No branch watcher or context is needed once direct status is selected.
			state, config, err := PhysicalStatus(t.Context(), pg)
			if err != nil || state != tt.want {
				t.Fatalf("status = %q, error = %v; want %q", state, err, tt.want)
			}
			wantConfig := &PostgresConfiguration{Parameters: []*PostgresParameter{{Name: "work_mem", Value: "32MB"}}}
			if diff := cmp.Diff(wantConfig, config); diff != "" {
				t.Errorf("configuration (-want +got):\n%s", diff)
			}
			branch := mainBranchView(pg)
			legacy := &PostgresBranch{Name: "main", PostgresName: pg.Name, TeamSlug: pg.TeamSlug, EnvironmentName: pg.EnvironmentName}
			if branch.ID() != legacy.ID() {
				t.Error("virtual main changed the legacy node ID")
			}
			if branch.clusterName() != *pg.directClusterName {
				t.Error("virtual main did not preserve the observed physical target")
			}
		})
	}
}

func TestDirectPostgresInvalidStatus(t *testing.T) {
	for _, status := range []map[string]any{
		{"clusterName": "invalid/target"},
		{"clusterName": int64(123)},
		{"clusterName": "pg-orders", "configuration": "invalid"},
		{"clusterName": "pg-orders", "configuration": map[string]any{"parameters": map[string]any{"work_mem": int64(32)}}},
	} {
		if _, err := toPostgres(physicalPostgresObject("orders", status), "dev"); err == nil {
			t.Errorf("accepted invalid physical status: %v", status)
		}
	}
}

func TestBranchViewsSelectDirectStatusWithoutDuplicates(t *testing.T) {
	pg := &Postgres{Name: "orders", TeamSlug: "team", EnvironmentName: "dev", directClusterName: new("pg-existing"), physicalState: PostgresBranchStateDegraded}
	branches := []*PostgresBranch{
		{Name: "main", PostgresName: "orders", TeamSlug: "team", EnvironmentName: "dev", State: PostgresBranchStateAvailable},
		{Name: "restore", PostgresName: "orders", TeamSlug: "team", EnvironmentName: "dev"},
		{Name: "main", PostgresName: "orders", TeamSlug: "team", EnvironmentName: "prod"},
		{Name: "main", PostgresName: "orders", TeamSlug: "other", EnvironmentName: "dev"},
		{Name: "main", PostgresName: "legacy", TeamSlug: "team", EnvironmentName: "dev"},
	}
	views := BranchViews([]*Postgres{pg}, branches)
	if len(views) != 4 {
		t.Fatalf("views = %d, want 4", len(views))
	}
	for _, branch := range views {
		if branch.PostgresName == pg.Name && branch.TeamSlug == pg.TeamSlug && branch.EnvironmentName == pg.EnvironmentName {
			if branch.Name != "main" || branch.State != PostgresBranchStateDegraded {
				t.Errorf("stale branch won over direct status: %+v", branch)
			}
		}
	}
	if !slices.Contains(views, branches[2]) || !slices.Contains(views, branches[3]) || !slices.Contains(views, branches[4]) {
		t.Error("source selection leaked across database, namespace or environment")
	}
}

func TestPostgresDualReadAcrossAdoption(t *testing.T) {
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
	client := mgr.GetDynamicClients()["dev"]
	pgClient := client.Resource(postgresGVR()).Namespace("team")
	obj, err := pgClient.Create(ctx, physicalPostgresObject("orders", nil), metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	branchObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
		"metadata": map[string]any{"name": nais_io_v1.PostgresPhysicalName("orders"), "namespace": "team", "generation": int64(2)},
		"spec":     map[string]any{"postgres": "orders", "branchName": "main"},
		"status":   healthyPhysicalStatus("pg-legacy"),
	}}
	branchClient := client.Resource(nais_io_v1.GroupVersion.WithResource("postgresbranches")).Namespace("team")
	if _, err := branchClient.Create(ctx, branchObj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	branches := NewPostgresBranchWatcher(ctx, mgr)
	postgreses := NewPostgresWatcher(ctx, mgr)
	ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
	defer cancelReady()
	if !mgr.WaitForReady(ready) {
		t.Fatal("watchers did not synchronize")
	}
	ctx = NewLoaderContext(ctx, branches, postgreses, "", "", "nav", mgr.GetDynamicClients())
	pg, err := GetPostgres(ctx, slug.Slug("team"), "dev", "orders")
	if err != nil {
		t.Fatal(err)
	}
	state, config, err := PhysicalStatus(ctx, pg)
	if err != nil || state != PostgresBranchStateAvailable || config == nil {
		t.Fatalf("legacy status = %q, %v, %v", state, config, err)
	}

	// Direct status is intentionally incomplete: stale healthy main must not win.
	obj.Object["status"] = map[string]any{"clusterName": "pg-preserved-main-12345678"}
	if _, err := pgClient.UpdateStatus(ctx, obj, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		pg, err := postgreses.Get("dev", "team", "orders")
		return err == nil && pg.directClusterName != nil, err
	}); err != nil {
		t.Fatal("Postgres watcher did not observe direct status:", err)
	}
	main, err := GetPostgresBranch(ctx, "team", "dev", "orders", "main")
	if err != nil || main.State != PostgresBranchStateProgressing || main.Configuration != nil {
		t.Fatalf("direct main = %+v, error = %v", main, err)
	}
	if _, err := GetPostgresBranch(ctx, "team", "dev", "orders", "restore"); !errors.Is(err, &watcher.ErrorNotFound{}) {
		t.Errorf("non-main unexpectedly resolved: %v", err)
	}
	if err := branchClient.Delete(ctx, branchObj.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		views := ListAllForTeam(ctx, "team", nil)
		return len(views) == 1 && views[0].ClusterName == "pg-preserved-main-12345678" && views[0].State == PostgresBranchStateProgressing, nil
	}); err != nil {
		t.Fatal("listing did not switch to direct status:", err)
	}
	main, err = GetPostgresBranchByObjectName(ctx, "team", "dev", branchObj.GetName())
	if err != nil || main.clusterName() != "pg-preserved-main-12345678" {
		t.Fatalf("legacy access reference after detach = %+v, error = %v", main, err)
	}

	// The access check must use the freshly observed direct target, not a
	// branch-name reconstruction or stale parent from the list informer.
	obj.Object["status"] = healthyPhysicalStatus("pg-preserved-main-12345678")
	if _, err := pgClient.UpdateStatus(ctx, obj, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
		"metadata": map[string]any{"name": "pg-preserved-main-12345678", "namespace": "team"},
		"status":   map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}},
	}}
	clustersClient := client.Resource(cnpgClusterGVR).Namespace("team")
	if _, err := clustersClient.Create(ctx, cluster, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	main, err = GetReadyPostgresBranch(ctx, "team", "dev", "orders", "main")
	if err != nil || main.State != PostgresBranchStateAvailable {
		t.Fatalf("direct ready access target = %+v, error = %v", main, err)
	}
	cluster.Object["status"] = map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "False"}}}
	if _, err := clustersClient.UpdateStatus(ctx, cluster, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	main, err = GetReadyPostgresBranch(ctx, "team", "dev", "orders", "main")
	if err != nil || main.State != PostgresBranchStateProgressing {
		t.Fatalf("access must not trust healthy parent over CNPG readiness: %+v, error = %v", main, err)
	}
}
