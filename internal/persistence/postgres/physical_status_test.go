package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		{name: "healthy", want: PostgresStateAvailable},
		{name: "old observed generation", change: func(s map[string]any) { s["observedGeneration"] = int64(1) }, want: PostgresStateProgressing},
		{name: "not completed", change: func(s map[string]any) { s["reconcilePhase"] = "Preparing" }, want: PostgresStateProgressing},
		{name: "no physical condition", change: func(s map[string]any) { delete(s, "conditions") }, want: PostgresStateProgressing},
		{name: "degraded", change: func(s map[string]any) {
			s["conditions"].([]any)[0].(map[string]any)["message"] = "Cluster is in phase: Cluster is unrecoverable and needs manual intervention"
		}, want: PostgresStateDegraded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			status := healthyPhysicalStatus("pg-existing-12345678")
			if tt.change != nil {
				tt.change(status)
			}
			pg, err := toPostgres(physicalPostgresObject("orders", status), "dev")
			if err != nil {
				t.Fatal(err)
			}
			state, config, err := PhysicalStatus(t.Context(), pg)
			if err != nil || state != tt.want {
				t.Fatalf("status = %q, error = %v; want %q", state, err, tt.want)
			}
			wantConfig := &PostgresConfiguration{Parameters: []*PostgresParameter{{Name: "work_mem", Value: "32MB"}}}
			if diff := cmp.Diff(wantConfig, config); diff != "" {
				t.Errorf("configuration (-want +got):\n%s", diff)
			}
			cluster, err := physicalClusterName(t.Context(), pg)
			if err != nil || cluster == nil || *cluster != "pg-existing-12345678" {
				t.Fatalf("physical target = %v, %v", cluster, err)
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

func TestPostgresAccessCorroboratesFreshPhysicalStatus(t *testing.T) {
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
	postgreses := NewPostgresWatcher(ctx, mgr)
	ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
	defer cancelReady()
	if !mgr.WaitForReady(ready) {
		t.Fatal("watchers did not synchronize")
	}
	ctx = NewLoaderContext(ctx, postgreses, "", "", "nav", mgr.GetDynamicClients())
	state, err := readyPostgresState(ctx, "team", "dev", "orders")
	if err != nil || state != PostgresStateProgressing {
		t.Fatalf("provisioning = %q, %v", state, err)
	}

	// Access reads must use fresh status, even before the informer catches up.
	obj.Object["status"] = healthyPhysicalStatus("pg-preserved-12345678")
	if _, err := pgClient.UpdateStatus(ctx, obj, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	state, err = readyPostgresState(ctx, "team", "dev", "orders")
	if err != nil || state != PostgresStateProgressing {
		t.Fatalf("missing cluster = %q, %v", state, err)
	}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
		"metadata": map[string]any{"name": "pg-preserved-12345678", "namespace": "team"},
		"status":   map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}},
	}}
	clustersClient := client.Resource(cnpgClusterGVR).Namespace("team")
	if _, err := clustersClient.Create(ctx, cluster, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	state, err = readyPostgresState(ctx, "team", "dev", "orders")
	if err != nil || state != PostgresStateAvailable {
		t.Fatalf("ready = %q, %v", state, err)
	}
	cluster.Object["status"] = map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "False"}}}
	if _, err := clustersClient.UpdateStatus(ctx, cluster, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	state, err = readyPostgresState(ctx, "team", "dev", "orders")
	if err != nil || state != PostgresStateProgressing {
		t.Fatalf("unready cluster = %q, %v", state, err)
	}
}
