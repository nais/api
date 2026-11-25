package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestAccessSelectionUsesFreshObservedParent(t *testing.T) {
	for _, available := range []bool{false, true} {
		t.Run(map[bool]string{false: "provisioning", true: "available"}[available], func(t *testing.T) {
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
			parent, err := pgClient.Create(ctx, physicalPostgresObject("orders", nil), metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			postgreses := NewPostgresWatcher(ctx, mgr)
			ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
			defer cancelReady()
			if !mgr.WaitForReady(ready) {
				t.Fatal("watchers did not synchronize")
			}
			if available {
				parent.Object["status"] = healthyPhysicalStatus("pg-observed")
				if _, err := pgClient.UpdateStatus(ctx, parent, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
				cluster := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
					"metadata": map[string]any{"name": "pg-observed", "namespace": "team"},
					"status":   map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}},
				}}
				if _, err := client.Resource(cnpgClusterGVR).Namespace("team").Create(ctx, cluster, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			ctx = NewLoaderContext(ctx, postgreses, nil, "", "", "nav", mgr.GetDynamicClients())
			parentGets := 0
			client.(*dynamicfake.FakeDynamicClient).PrependReactor("get", "postgres", func(k8stesting.Action) (bool, runtime.Object, error) {
				parentGets++
				return false, nil, nil
			})
			input := CreatePostgresAccessInput{Postgres: "orders", TeamSlug: "team", EnvironmentName: "dev", AccessLevel: PostgresAccessLevelRead, Reason: "Investigating access selection"}
			err = input.Validate(ctx)
			if (err == nil) != available {
				t.Fatalf("available=%t: validation = %v", available, err)
			}
			if parentGets != 1 {
				t.Fatalf("access selection used %d parent reads, want 1", parentGets)
			}
			resource := newPostgresAccessResource(input, "user@example.com", "access", time.Now().Add(time.Hour))
			got, _, err := unstructured.NestedString(resource.Object, "spec", "postgres")
			if err != nil || got != "orders" {
				t.Fatalf("access target = %q, %v", got, err)
			}
		})
	}
}
