package watchers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/slug"
	v1 "github.com/nais/pgrator/pkg/api/v1"
	logtest "github.com/sirupsen/logrus/hooks/test"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestPostgresContractsWithoutBranchAPI(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logtest.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	c := mgr.GetDynamicClients()["dev"].(*dynamicfake.FakeDynamicClient)
	var branchCalls atomic.Int32
	c.PrependReactor("*", "postgresbranches", func(k8stesting.Action) (bool, runtime.Object, error) {
		branchCalls.Add(1)
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "nais.io", Resource: "postgresbranches"}, "")
	})
	c.PrependWatchReactor("postgresbranches", func(k8stesting.Action) (bool, watch.Interface, error) {
		branchCalls.Add(1)
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "nais.io", Resource: "postgresbranches"}, "")
	})
	clusterName := v1.CNPGClusterName(v1.PostgresPhysicalName("orders"))
	pg := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "nais.io/v1", "kind": "Postgres", "metadata": map[string]any{"name": "orders", "namespace": "team", "generation": int64(1)}, "spec": map[string]any{"majorVersion": "18"}, "status": map[string]any{"clusterName": clusterName, "reconcilePhase": "Completed", "observedGeneration": int64(1), "conditions": []any{map[string]any{"type": "cluster.postgresql.cnpg.io/ObservedState", "status": "True", "message": "Cluster is in phase: Cluster in healthy state"}}}}}
	if _, err := c.Resource(v1.GroupVersion.WithResource("postgres")).Namespace("team").Create(ctx, pg, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	cnpg := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster", "metadata": map[string]any{"name": clusterName, "namespace": "team"}, "status": map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}}}}
	if _, err := c.Resource(schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}).Namespace("team").Create(ctx, cnpg, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	registered := watchers.SetupWatchers(ctx, mgr, nil, false)
	ready, cancelReady := context.WithTimeout(ctx, 10*time.Second)
	defer cancelReady()
	if !mgr.WaitForReady(ready) {
		t.Fatal("watchers did not synchronize without branch API")
	}
	ctx = postgres.NewLoaderContext(ctx, registered.PostgresWatcher, "", "", "nav", mgr.GetDynamicClients())
	views := postgres.ListAllForTeam(ctx, "team", nil)
	if len(views) != 1 || views[0].Name != "main" || views[0].State != postgres.PostgresBranchStateAvailable {
		t.Fatalf("main facade = %+v", views)
	}
	if _, err := postgres.GetPostgresBranchByIdent(ctx, views[0].ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.GetPostgresBranchByObjectName(ctx, "team", "dev", v1.PostgresPhysicalName("orders")); err != nil {
		t.Fatal(err)
	}
	if parent, err := postgres.GetPostgresByPhysicalName(ctx, "team", "dev", v1.PostgresPhysicalName("orders")); err != nil || parent.Name != "orders" {
		t.Fatalf("direct issue relation: %+v, %v", parent, err)
	}
	for _, scope := range []struct{ team, env string }{{"other-team", "dev"}, {"team", "prod"}} {
		if _, err := postgres.GetPostgresByPhysicalName(ctx, slug.Slug(scope.team), scope.env, v1.PostgresPhysicalName("orders")); !errors.Is(err, &watcher.ErrorNotFound{}) {
			t.Errorf("legacy issue lookup escaped scope %+v: %v", scope, err)
		}
	}
	if selected, err := postgres.GetReadyPostgresBranch(ctx, "team", "dev", "orders", ""); err != nil || selected.State != postgres.PostgresBranchStateAvailable {
		t.Fatalf("access target = %+v, %v", selected, err)
	}
	proxy := postgres.WithLogScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	parent, err := postgres.GetPostgres(ctx, "team", "dev", "orders")
	if err != nil {
		t.Fatal(err)
	}
	parameters := url.Values{"postgres": {parent.ID().String()}, "query": {`{service_name="orders"}`}}
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/loki/api/v1/query_range?"+parameters.Encode(), nil).WithContext(ctx))
	if response.Code != http.StatusNoContent {
		t.Fatalf("log request failed: %d %s", response.Code, response.Body.String())
	}
	if got := branchCalls.Load(); got != 0 {
		t.Fatalf("used retired branch API %d times", got)
	}
}
