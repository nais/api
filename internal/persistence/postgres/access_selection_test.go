package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestAccessSelectionUsesFreshObservedParent(t *testing.T) {
	for _, tc := range []struct {
		name, requested, observed, explicit, want string
		direct                                    bool
		wantError                                 bool
	}{
		{name: "omitted ignores stale active", observed: "recovered", direct: true, want: "main"},
		{name: "desired activation does not select target", requested: "main", observed: "recovered", direct: true, want: "main"},
		{name: "explicit main remains explicit", observed: "recovered", explicit: "main", direct: true, want: "main"},
		{name: "explicit retired branch is rejected", observed: "main", explicit: "recovered", direct: true, wantError: true},
		{name: "no observed direct status is unavailable", requested: "recovered", wantError: true},
		{name: "direct ownership uses virtual main", observed: "recovered", direct: true, want: "main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			// Synchronize a different parent first. The access decision must use
			// the fresh API-server value written below, not this cached selection.
			parent := physicalPostgresObject("orders", map[string]any{"activeBranch": "main"})
			parent.Object["spec"].(map[string]any)["activeBranch"] = tc.requested
			parent, err = pgClient.Create(ctx, parent, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"main", "recovered"} {
				branch := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
					"metadata": map[string]any{"name": nais_io_v1.PostgresBranchObjectName("orders", name), "namespace": "team", "generation": int64(2)},
					"spec":     map[string]any{"postgres": "orders", "branchName": name},
					"status":   healthyPhysicalStatus("unused"),
				}}
				if _, err := client.Resource(nais_io_v1.GroupVersion.WithResource("postgresbranches")).Namespace("team").Create(ctx, branch, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
				cluster := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
					"metadata": map[string]any{"name": nais_io_v1.CNPGClusterName(branch.GetName()), "namespace": "team"},
					"status":   map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}},
				}}
				if _, err := client.Resource(cnpgClusterGVR).Namespace("team").Create(ctx, cluster, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			postgreses := NewPostgresWatcher(ctx, mgr)
			ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
			defer cancelReady()
			if !mgr.WaitForReady(ready) {
				t.Fatal("watchers did not synchronize")
			}
			status := map[string]any{}
			if tc.observed != "" {
				status["activeBranch"] = tc.observed
			}
			if tc.direct {
				status = healthyPhysicalStatus(nais_io_v1.CNPGClusterName(nais_io_v1.PostgresPhysicalName("orders")))
				status["activeBranch"] = tc.observed
			}
			parent.Object["status"] = status
			if _, err := pgClient.UpdateStatus(ctx, parent, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			ctx = NewLoaderContext(ctx, postgreses, "", "", "nav", mgr.GetDynamicClients())
			parentGets := 0
			client.(*dynamicfake.FakeDynamicClient).PrependReactor("get", "postgres", func(k8stesting.Action) (bool, runtime.Object, error) {
				parentGets++
				return false, nil, nil
			})
			input := CreatePostgresAccessInput{Postgres: "orders", Branch: tc.explicit, TeamSlug: "team", EnvironmentName: "dev", AccessLevel: PostgresAccessLevelRead, Reason: "Investigating access selection"}
			err = input.Validate(ctx)
			if tc.wantError {
				if err == nil {
					t.Fatal("accepted retired or unready target")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if parentGets != 1 {
				t.Fatalf("access selection used %d fresh parent reads, want exactly one", parentGets)
			}
			if input.Branch != tc.want {
				t.Fatalf("selected %q, want %q", input.Branch, tc.want)
			}
			// The resolved choice is also the immutable creation identity; do not
			// resolve it again or replace an explicit request during creation.
			resource := newPostgresAccessResource(input, "user@example.com", "access", time.Now().Add(time.Hour))
			got, _, err := unstructured.NestedString(resource.Object, "spec", "postgres")
			if err != nil || got != "orders" {
				t.Fatalf("created access target = %q, error = %v", got, err)
			}
		})
	}
}
