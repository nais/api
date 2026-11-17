package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestIngressCacheSyncDoesNotWaitForOtherResources(t *testing.T) {
	log, _ := logrustest.NewNullLogger()
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := watcher.NewManager(scheme, kubernetes.ClusterConfigMap{"dev": nil, "prod": nil}, log, watcher.WithClientCreator(fake.Clients(nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Stop()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	appWatcher := watcher.Watch(mgr, &nais_io_v1alpha1.Application{})
	ingressWatcher := NewIngressWatcher(ctx, mgr)
	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
	defer waitCancel()
	if !ingressWatcher.WaitForCacheSync(waitCtx) {
		t.Fatal("ingress synchronization blocked on an unrelated application watcher")
	}
	if mgr.IsReady() {
		t.Fatal("manager should not be ready while the application watcher is not running")
	}
	for _, env := range ingressWatcher.EnvironmentNames() {
		if !ingressWatcher.HasSynced(env) {
			t.Errorf("ingress cache is not synchronized for %s", env)
		}
	}

	cancel()
	if appWatcher.WaitForCacheSync(ctx) {
		t.Fatal("canceled wait for an unsynchronized cache should fail")
	}
}

func TestIndexIngressHostsSkipsHostlessRules(t *testing.T) {
	for _, tt := range []struct {
		name  string
		rules []any
		want  []string
	}{
		{name: "no rules", want: []string{}},
		{name: "absent host", rules: []any{map[string]any{}}, want: []string{}},
		{name: "null host", rules: []any{map[string]any{"host": nil}}, want: []string{}},
		{name: "empty host", rules: []any{map[string]any{"host": ""}}, want: []string{}},
		{
			name: "hostless and named rules",
			rules: []any{
				map[string]any{},
				map[string]any{"host": nil},
				map[string]any{"host": "APP.EXAMPLE.COM."},
			},
			want: []string{"app.example.com"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "networking.k8s.io/v1",
				"kind":       "Ingress",
				"spec":       map[string]any{},
			}}
			if tt.rules != nil {
				if err := unstructured.SetNestedSlice(ingress.Object, tt.rules, "spec", "rules"); err != nil {
					t.Fatal(err)
				}
			}
			transformed, err := transformIngress(ingress)
			if err != nil {
				t.Fatal(err)
			}
			hosts, err := indexIngressHosts(transformed)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, hosts); diff != "" {
				t.Errorf("unexpected indexed hosts (-want +got):\n%s", diff)
			}
		})
	}
}
