package postgres

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/loki"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestScopedPostgresLogQuery(t *testing.T) {
	pg := &Postgres{Name: "orders", TeamSlug: "team", EnvironmentName: "dev"}
	for _, query := range []string{
		`{service_name="orders"}`,
		`{service_name="orders"} |= "failed" | json | level="ERROR"`,
		`{service_name="orders"} | label_format k8s_pod_name="pg-other-1"`,
		`sum(count_over_time({service_name="orders"}[1m]))`,
		`sum(count_over_time({service_name="orders"} | json [5m]))`,
		`count_over_time({service_name="orders"}[1m]) + count_over_time({service_name="other"}[1m])`,
	} {
		t.Run(query, func(t *testing.T) {
			scoped, err := scopedLogQuery(query, pg, "pg-orders.api-main-a4f04c0c")
			if err != nil {
				t.Fatal(err)
			}
			expr, err := syntax.ParseExpr(scoped)
			if err != nil {
				t.Fatalf("rewritten query is invalid: %v\n%s", err, scoped)
			}
			selectors, pipelines := 0, 0
			expr.Walk(func(e syntax.Expr) bool {
				switch e := e.(type) {
				case *syntax.MatchersExpr:
					selectors++
					for name, value := range map[string]string{"service_namespace": "team", "service_name": "orders", "k8s_cluster_name": "dev"} {
						found := false
						for _, matcher := range e.Matchers() {
							if matcher.Name == name && matcher.String() == name+`="`+value+`"` {
								found = true
							}
						}
						if !found {
							t.Errorf("selector lacks exact %s=%q: %s", name, value, scoped)
						}
					}
				case *syntax.PipelineExpr:
					pipelines++
					if len(e.MultiStages) < 2 || e.MultiStages[0].String() != `| k8s_container_name="postgres"` ||
						e.MultiStages[1].String() != "| k8s_pod_name=~`^pg-orders\\.api-main-a4f04c0c-[0-9]+$`" {
						t.Errorf("physical scope must precede all user stages: %s", scoped)
					}
				}
				return true
			})
			if selectors == 0 || pipelines != selectors {
				t.Errorf("not every selector has a scoped pipeline: %s", scoped)
			}
		})
	}
	for _, query := range []string{"invalid", "vector(1)", "1 + 2"} {
		if _, err := scopedLogQuery(query, pg, "pg-orders"); err == nil {
			t.Errorf("accepted query without valid log streams: %q", query)
		}
	}
}

func TestPostgresLogInstancePattern(t *testing.T) {
	for _, cluster := range []string{"pg-orders-main-a4f04c0c", "pg-orders.api", "preserved-name"} {
		query, err := scopedLogQuery(`{service_name="orders"}`, &Postgres{Name: "orders", TeamSlug: "team", EnvironmentName: "dev"}, cluster)
		if err != nil {
			t.Fatal(err)
		}
		expr, err := syntax.ParseExpr(query)
		if err != nil {
			t.Fatal(err)
		}
		_, encoded, ok := strings.Cut(expr.(*syntax.PipelineExpr).MultiStages[1].String(), "=~")
		if !ok {
			t.Fatalf("missing instance filter: %s", query)
		}
		patternText, err := strconv.Unquote(encoded)
		if err != nil {
			t.Fatal(err)
		}
		pattern := regexp.MustCompile(patternText)
		for _, serial := range []string{"1", "2", "10"} {
			if !pattern.MatchString(cluster + "-" + serial) {
				t.Error("instance excluded")
			}
		}
		for _, pod := range []string{cluster, cluster + "-pooler-abc", cluster + "-feature-1", cluster + "-1-extra", "other-" + cluster + "-1", cluster + "2-1"} {
			if pattern.MatchString(pod) {
				t.Errorf("non-instance matched: %s", pod)
			}
		}
	}
}

func TestPostgresLogProxy(t *testing.T) {
	ctx := postgresLogTestContext(t)
	for _, tc := range []struct {
		name, database, path, query, method, rawID, wantCluster string
		wantStatus                                              int
	}{
		{name: "missing direct status log page", database: "legacy", wantStatus: http.StatusConflict},
		{name: "direct log page", database: "direct", wantCluster: "pg-preserved-main-12345678"},
		{name: "direct volume", database: "direct", path: "query", query: `sum(count_over_time({service_name="direct"}[1m]))`, wantCluster: "pg-preserved-main-12345678"},
		{name: "missing direct status volume", database: "legacy", path: "query", query: `sum(count_over_time({service_name="legacy"}[1m]))`, wantStatus: http.StatusConflict},
		{name: "incomplete direct beats stale legacy", database: "incomplete", wantCluster: "pg-direct-incomplete"},
		{name: "unresolved must not query broad logs", database: "pending", wantStatus: http.StatusConflict},
		{name: "missing database", database: "missing", wantStatus: http.StatusNotFound},
		{name: "invalid ID", rawID: "invalid", wantStatus: http.StatusBadRequest},
		{name: "wrong node type", rawID: newAccessIdent("team", "dev", "direct").String(), wantStatus: http.StatusBadRequest},
		{name: "empty ID", wantStatus: http.StatusBadRequest},
		{name: "empty identity part", rawID: newPostgresIdent("", "dev", "direct").String(), wantStatus: http.StatusBadRequest},
		{name: "invalid query", database: "direct", query: "invalid", wantStatus: http.StatusBadRequest},
		{name: "unsupported path", database: "direct", path: "series", wantStatus: http.StatusBadRequest},
		{name: "unsupported method", database: "direct", method: http.MethodPost, wantStatus: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := tc.rawID
			if tc.database != "" {
				id = newPostgresIdent("team", "dev", tc.database).String()
			}
			path, method, query := tc.path, tc.method, tc.query
			if path == "" {
				path = "query_range"
			}
			if method == "" {
				method = http.MethodGet
			}
			if query == "" {
				query = `{service_name="` + tc.database + `"} |= "failed"`
			}
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				params := r.URL.Query()
				if params.Has("postgres") || params.Get("limit") != "500" || params.Get("start") != "123" || params.Get("end") != "456" || r.URL.Path != "/loki/api/v1/"+path {
					t.Errorf("unexpected upstream parameters: %s", r.URL)
				}
				if !strings.Contains(params.Get("query"), tc.wantCluster+"-[0-9]+$") || strings.Contains(params.Get("query"), "pg-stale") {
					t.Errorf("wrong physical target: %s", params.Get("query"))
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer upstream.Close()
			log, _ := logrustest.NewNullLogger()
			proxy, err := loki.NewProxy(upstream.URL, log)
			if err != nil {
				t.Fatal(err)
			}
			params := url.Values{"postgres": {id}, "query": {query}, "limit": {"500"}, "start": {"123"}, "end": {"456"}}
			request := httptest.NewRequest(method, "/loki/api/v1/"+path+"?"+params.Encode(), nil).WithContext(ctx)
			response := httptest.NewRecorder()
			WithLogScope(proxy).ServeHTTP(response, request)
			wantStatus, wantCalls := tc.wantStatus, 0
			if wantStatus == 0 {
				wantStatus, wantCalls = http.StatusNoContent, 1
			}
			if response.Code != wantStatus || calls != wantCalls {
				t.Errorf("status=%d, upstream calls=%d; want %d/%d: %s", response.Code, calls, wantStatus, wantCalls, response.Body.String())
			}
		})
	}
}

func TestPostgresLogProxyLeavesOrdinaryRequestsUnchanged(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/loki/api/v1/query_range?query=unchanged", nil)
	called := false
	WithLogScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r != request {
			t.Error("ordinary proxy request changed")
		}
	})).ServeHTTP(httptest.NewRecorder(), request)
	if !called {
		t.Fatal("ordinary Loki request was not forwarded")
	}
}

func postgresLogTestContext(t *testing.T) context.Context {
	t.Helper()
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
	for _, pg := range []*unstructured.Unstructured{
		physicalPostgresObject("legacy", nil),
		physicalPostgresObject("direct", healthyPhysicalStatus("pg-preserved-main-12345678")),
		physicalPostgresObject("incomplete", map[string]any{"clusterName": "pg-direct-incomplete"}),
		physicalPostgresObject("pending", nil),
	} {
		if _, err := client.Resource(postgresGVR()).Namespace("team").Create(ctx, pg, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, database := range []string{"legacy", "incomplete"} {
		cluster := "pg-stale"
		if database == "legacy" {
			cluster = "pg-legacy-main-a4f04c0c"
		}
		branch := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
			"metadata": map[string]any{"name": nais_io_v1.PostgresPhysicalName(database), "namespace": "team", "generation": int64(2)},
			"spec":     map[string]any{"postgres": database, "branchName": "main"}, "status": healthyPhysicalStatus(cluster),
		}}
		if _, err := client.Resource(nais_io_v1.GroupVersion.WithResource("postgresbranches")).Namespace("team").Create(ctx, branch, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	postgreses := NewPostgresWatcher(ctx, mgr)
	ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
	defer cancelReady()
	if !mgr.WaitForReady(ready) {
		t.Fatal("watchers did not synchronize")
	}
	return NewLoaderContext(ctx, postgreses, "", "", "nav", mgr.GetDynamicClients())
}
