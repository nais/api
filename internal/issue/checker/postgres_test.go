package checker

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/postgres"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	"github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestPostgresWithoutWatcher(t *testing.T) {
	for _, tt := range []struct {
		name  string
		check Postgres
	}{
		{name: "nil watcher"},
		{name: "disabled watcher", check: Postgres{PostgresBranchWatcher: &watcher.Watcher[*postgres.PostgresBranch]{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := tt.check.Run(t.Context())
			if err != nil || len(issues) != 0 {
				t.Fatalf("expected no issues or error, got %v, %v", issues, err)
			}
		})
	}
}

func TestPostgresBranchIssues(t *testing.T) {
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
	client := mgr.GetDynamicClients()["dev"].Resource(nais_io_v1.GroupVersion.WithResource("postgresbranches")).Namespace("team")
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
		"metadata": map[string]any{"name": nais_io_v1.PostgresBranchObjectName("orders", "restore"), "namespace": "team", "generation": int64(1)},
		"spec":     map[string]any{"postgres": "orders", "branchName": "restore"},
	}}
	if _, err := client.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	w := postgres.NewPostgresBranchWatcher(ctx, mgr)
	ready, cancelReady := context.WithTimeout(ctx, 5*time.Second)
	defer cancelReady()
	if !mgr.WaitForReady(ready) {
		t.Fatal("PostgresBranch watcher did not synchronize")
	}
	check := Postgres{PostgresBranchWatcher: w}
	want := []Issue{{
		IssueType:    issue.IssueTypePostgresBranch,
		ResourceName: obj.GetName(),
		ResourceType: issue.ResourceTypePostgresBranch,
		Team:         "team",
		Env:          "dev",
		Severity:     issue.SeverityCritical,
		Message:      `Postgres branch "restore" of "orders" has reported a failure.`,
	}}
	for _, tt := range []struct {
		name   string
		status map[string]any
		state  postgres.PostgresBranchState
		want   []Issue
	}{
		{name: "not observed", state: postgres.PostgresBranchStateProgressing},
		{name: "recovery failed on an inactive branch", status: map[string]any{
			"reconcilePhase": "Failed",
			"conditions":     []any{map[string]any{"type": "RecoveryFailed", "status": "True", "reason": "Unrecoverable"}},
		}, state: postgres.PostgresBranchStateDegraded, want: want},
		{name: "healthy again", status: map[string]any{
			"reconcilePhase": "Completed", "observedGeneration": int64(1),
			"conditions": []any{map[string]any{"type": "cluster.postgresql.cnpg.io/ObservedState", "status": "True", "message": "Cluster is in phase: Cluster in healthy state"}},
		}, state: postgres.PostgresBranchStateAvailable},
		{name: "progressing again", state: postgres.PostgresBranchStateProgressing},
		{name: "cluster failure", status: map[string]any{
			"reconcilePhase": "Completed", "observedGeneration": int64(1),
			"conditions": []any{map[string]any{"type": "cluster.postgresql.cnpg.io/ObservedState", "status": "True", "message": "Cluster is in phase: Cluster is unrecoverable and needs manual intervention"}},
		}, state: postgres.PostgresBranchStateDegraded, want: want},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj.Object["status"] = tt.status
			if _, err := client.UpdateStatus(ctx, obj, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
				branch, err := w.Get("dev", "team", obj.GetName())
				return err == nil && branch.State == tt.state, err
			}); err != nil {
				t.Fatal(err)
			}
			got, err := check.Run(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("issues mismatch (-want +got):\n%s", diff)
			}
		})
	}
	if err := client.Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) {
		return len(w.All()) == 0, nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := check.Run(ctx); err != nil || len(got) != 0 {
		t.Fatalf("deleted branch should have no issues, got %v, %v", got, err)
	}
}
