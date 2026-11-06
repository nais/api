package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	"github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestPostgresBranchConfigurationFromStatus(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  map[string]any
		want    *PostgresConfiguration
		wantErr bool
	}{
		{name: "no status from older pgrator"},
		{name: "no configuration from older pgrator", status: map[string]any{"reconcilePhase": "Completed"}},
		{name: "null configuration", status: map[string]any{"configuration": nil}},
		{
			name: "empty parameters", status: map[string]any{"configuration": map[string]any{"parameters": map[string]any{}}},
			want: &PostgresConfiguration{Parameters: []*PostgresParameter{}},
		},
		{
			name: "missing parameters", status: map[string]any{"configuration": map[string]any{}},
			want: &PostgresConfiguration{Parameters: []*PostgresParameter{}},
		},
		{
			name: "raw values sorted by name", status: map[string]any{"configuration": map[string]any{"parameters": map[string]any{
				"work_mem": "32MB", "pgaudit.log": "read,write,ddl,role", "checkpoint_timeout": "10min",
			}}},
			want: &PostgresConfiguration{Parameters: []*PostgresParameter{
				{Name: "checkpoint_timeout", Value: "10min"},
				{Name: "pgaudit.log", Value: "read,write,ddl,role"},
				{Name: "work_mem", Value: "32MB"},
			}},
		},
		{
			name: "invalid configuration", status: map[string]any{"configuration": "invalid"}, wantErr: true,
		},
		{
			name: "invalid parameters", status: map[string]any{"configuration": map[string]any{"parameters": "invalid"}}, wantErr: true,
		},
		{
			name: "non-string parameter", status: map[string]any{"configuration": map[string]any{"parameters": map[string]any{"work_mem": int64(32)}}}, wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
				"metadata": map[string]any{"name": nais_io_v1.PostgresBranchObjectName("orders", "main"), "namespace": "team"},
				"spec":     map[string]any{"postgres": "orders", "branchName": "main"},
			}}
			if tt.status != nil {
				obj.Object["status"] = tt.status
			}
			branch, err := toPostgresBranch(obj, "dev")
			if (err != nil) != tt.wantErr {
				t.Fatalf("toPostgresBranch error = %v, want error %t", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.want, branch.Configuration); diff != "" {
				t.Errorf("configuration mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPostgresBranchWatcherConfigurationUpdates(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := test.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
		"metadata": map[string]any{"name": nais_io_v1.PostgresBranchObjectName("orders", "main"), "namespace": "team", "generation": int64(1)},
		"spec":     map[string]any{"postgres": "orders", "branchName": "main"},
	}}
	client := mgr.GetDynamicClients()["dev"].Resource(nais_io_v1.GroupVersion.WithResource("postgresbranches")).Namespace("team")
	if _, err := client.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	w := NewPostgresBranchWatcher(ctx, mgr)
	ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
	defer cancelReady()
	if !mgr.WaitForReady(ready) {
		t.Fatal("PostgresBranch watcher did not synchronize")
	}
	for _, tt := range []struct {
		parameters map[string]any
		want       *PostgresConfiguration
	}{
		{parameters: map[string]any{"work_mem": "8MB"}, want: &PostgresConfiguration{Parameters: []*PostgresParameter{{Name: "work_mem", Value: "8MB"}}}},
		{parameters: map[string]any{}, want: &PostgresConfiguration{Parameters: []*PostgresParameter{}}},
		{},
	} {
		obj.Object["status"] = map[string]any{}
		if tt.parameters != nil {
			obj.Object["status"] = map[string]any{"configuration": map[string]any{"parameters": tt.parameters}}
		}
		// Status updates do not change the resource generation.
		if _, err := client.UpdateStatus(ctx, obj, metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
			branch, err := w.Get("dev", "team", obj.GetName())
			return err == nil && cmp.Equal(tt.want, branch.Configuration), err
		}); err != nil {
			t.Fatalf("watcher did not observe configuration %#v: %v", tt.want, err)
		}
	}
}
