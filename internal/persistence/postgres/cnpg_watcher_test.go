package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCNPGClusterWatcher(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev", "prod"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := test.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clients := mgr.GetDynamicClients()
	// Parameters set by pgrator for a new cluster requesting 512Mi of memory.
	parameters := map[string]string{
		"log_min_duration_statement": "1000",
		"shared_buffers":             "128MB",
		"effective_cache_size":       "384MB",
		"work_mem":                   "8MB",
		"maintenance_work_mem":       "64MB",
		"random_page_cost":           "1.1",
		"effective_io_concurrency":   "200",
		"huge_pages":                 "off",
		"track_io_timing":            "on",
		"commit_delay":               "100",
		"commit_siblings":            "10",
		"max_wal_size":               "2GB",
		"wal_compression":            "zstd",
		"checkpoint_timeout":         "10min",
		"bgwriter_lru_maxpages":      "200",
		"pg_stat_statements.track":   "all",
		"pgaudit.log":                "read,write,ddl,role",
		"pgaudit.log_parameter":      "on",
		"pgaudit.log_relation":       "on",
	}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata":   map[string]any{"name": "orders-main", "namespace": "orders-team"},
	}}
	if err := unstructured.SetNestedStringMap(cluster.Object, parameters, "spec", "postgresql", "parameters"); err != nil {
		t.Fatal(err)
	}
	resource := clients["dev"].Resource(cnpgClusterGVR).Namespace(cluster.GetNamespace())
	if _, err := resource.Create(ctx, cluster, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	w := NewCNPGClusterWatcher(ctx, mgr)
	ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
	defer cancelReady()
	if !mgr.WaitForReady(ready) {
		t.Fatal("CNPG watcher did not synchronize")
	}
	if !w.Enabled() {
		t.Fatal("CNPG watcher should be enabled")
	}
	get := func() *CNPGCluster {
		t.Helper()
		obj, err := w.Get("dev", cluster.GetNamespace(), cluster.GetName())
		if err != nil {
			t.Fatal(err)
		}
		return obj
	}
	if diff := cmp.Diff(parameters, get().Parameters); diff != "" {
		t.Fatalf("cached parameters mismatch (-want +got):\n%s", diff)
	}
	for _, lookup := range []struct{ environment, namespace string }{
		{"prod", cluster.GetNamespace()},
		{"dev", "other-team"},
	} {
		if _, err := w.Get(lookup.environment, lookup.namespace, cluster.GetName()); !errors.Is(err, &watcher.ErrorNotFound{}) {
			t.Errorf("lookup in %s/%s: got %v, want not found", lookup.environment, lookup.namespace, err)
		}
	}

	// Mutating a returned object must not change the shared cache.
	get().Parameters["shared_buffers"] = "changed"
	if diff := cmp.Diff(parameters, get().Parameters); diff != "" {
		t.Fatalf("cache changed through returned object (-want +got):\n%s", diff)
	}

	// Observe new resources, updates and deletions through the same cache.
	cluster.SetName("orders-restore")
	if _, err := resource.Create(ctx, cluster, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForCNPGCache(t, func() bool {
		_, err := w.Get("dev", cluster.GetNamespace(), cluster.GetName())
		return err == nil
	})
	parameters["shared_buffers"] = "256MB"
	if err := unstructured.SetNestedStringMap(cluster.Object, parameters, "spec", "postgresql", "parameters"); err != nil {
		t.Fatal(err)
	}
	cluster.SetResourceVersion("2")
	if _, err := resource.Update(ctx, cluster, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForCNPGCache(t, func() bool {
		return cmp.Equal(parameters, get().Parameters)
	})
	if err := resource.Delete(ctx, cluster.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForCNPGCache(t, func() bool {
		_, err := w.Get("dev", cluster.GetNamespace(), cluster.GetName())
		return errors.Is(err, &watcher.ErrorNotFound{})
	})
}

func waitForCNPGCache(t *testing.T, check func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for CNPG cache change")
		case <-ticker.C:
		}
	}
}

func TestToCNPGClusterParameters(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    map[string]any
		want    map[string]string
		wantErr bool
	}{
		{name: "missing configuration"},
		{name: "missing parameters", spec: map[string]any{"postgresql": map[string]any{}}},
		{
			name: "empty parameters", spec: map[string]any{"postgresql": map[string]any{"parameters": map[string]any{}}},
			want: map[string]string{},
		},
		{
			name: "additional parameter is preserved",
			spec: map[string]any{"postgresql": map[string]any{"parameters": map[string]any{"statement_timeout": "5min"}}},
			want: map[string]string{"statement_timeout": "5min"},
		},
		{
			name:    "invalid parameter value",
			spec:    map[string]any{"postgresql": map[string]any{"parameters": map[string]any{"work_mem": int64(8)}}},
			wantErr: true,
		},
		{
			name:    "invalid parameters map",
			spec:    map[string]any{"postgresql": map[string]any{"parameters": "invalid"}},
			wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "postgresql.cnpg.io/v1",
				"kind":       "Cluster",
				"metadata": map[string]any{
					"name": "orders-main", "namespace": "orders-team",
					"labels": map[string]any{"postgres.nais.io/name": "orders-main"},
				},
			}}
			if tt.spec != nil {
				obj.Object["spec"] = tt.spec
			}
			got, err := toCNPGCluster(obj)
			if (err != nil) != tt.wantErr {
				t.Fatalf("toCNPGCluster error = %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.GetName() != obj.GetName() || got.GetNamespace() != obj.GetNamespace() {
				t.Errorf("cluster identity = %s/%s, want %s/%s", got.GetNamespace(), got.GetName(), obj.GetNamespace(), obj.GetName())
			}
			if diff := cmp.Diff(obj.GetLabels(), got.GetLabels()); diff != "" {
				t.Errorf("labels mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.want, got.Parameters); diff != "" {
				t.Errorf("parameters mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCNPGClusterDeepCopy(t *testing.T) {
	cluster := &CNPGCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-main", Labels: map[string]string{"postgres.nais.io/name": "orders-main"}},
		Parameters: map[string]string{"shared_buffers": "128MB"},
	}
	copy := cluster.DeepCopyObject().(*CNPGCluster)
	if diff := cmp.Diff(cluster, copy); diff != "" {
		t.Fatalf("copy mismatch (-want +got):\n%s", diff)
	}
	copy.Labels["postgres.nais.io/name"] = "changed"
	copy.Parameters["shared_buffers"] = "256MB"
	if cluster.Labels["postgres.nais.io/name"] != "orders-main" || cluster.Parameters["shared_buffers"] != "128MB" {
		t.Fatal("mutating copy changed original cluster")
	}
}
