package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	"github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCreateBranchRejectsInvalidRecoveryRequest(t *testing.T) {
	base := CreatePostgresBranchInput{Postgres: "foobar", Branch: "recovery", SourceBranch: "main", TargetTime: time.Now().Add(-time.Hour).Truncate(time.Second), EnvironmentName: "dev", TeamSlug: "someteamname"}
	tests := []struct {
		name   string
		change func(*CreatePostgresBranchInput)
	}{
		{"future recovery", func(i *CreatePostgresBranchInput) { i.TargetTime = time.Now().Add(time.Hour) }},
		{"missing target", func(i *CreatePostgresBranchInput) { i.TargetTime = time.Time{} }},
		{"same source and destination", func(i *CreatePostgresBranchInput) { i.Branch = "main" }},
		{"invalid destination", func(i *CreatePostgresBranchInput) { i.Branch = "UPPER_CASE" }},
		{"invalid source", func(i *CreatePostgresBranchInput) { i.SourceBranch = "bad/source" }},
		{"fractional target", func(i *CreatePostgresBranchInput) {
			i.TargetTime = time.Now().Add(-time.Hour).Truncate(time.Second).Add(time.Millisecond)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := base
			tt.change(&input)
			if _, err := CreateBranch(context.Background(), input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBranchOperationsRejectMissingBranches(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := test.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(os.DirFS("../../../integration_tests/k8s_resources/create_postgres_access"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	ctx := context.Background()
	branchWatcher := NewPostgresBranchWatcher(ctx, mgr)
	postgresWatcher := NewPostgresWatcher(ctx, mgr)
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("watchers did not synchronize")
	}
	ctx = NewLoaderContext(ctx, branchWatcher, postgresWatcher, "", "", "nav", mgr.GetDynamicClients())
	team := slug.Slug("someteamname")
	for _, tt := range []struct{ postgres, branch, want string }{
		{"foobar", "missing", "not found"},
		{"progressing", "recovered", "not found"},
	} {
		_, err := ActivateBranch(ctx, ActivatePostgresBranchInput{Postgres: tt.postgres, Branch: tt.branch, TeamSlug: team, EnvironmentName: "dev"})
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("activate %s/%s: %v; want %s", tt.postgres, tt.branch, err, tt.want)
		}
	}
	_, err = CreateBranch(ctx, CreatePostgresBranchInput{Postgres: "foobar", Branch: "new", SourceBranch: "missing", TargetTime: time.Now().Add(-time.Hour).Truncate(time.Second), TeamSlug: team, EnvironmentName: "dev"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("create with missing source: %v", err)
	}

	// A repeated activation of an already requested ready branch returns the
	// current desired and observed selection without writing another update.
	client, err := postgresClient(ctx, "dev", team)
	if err != nil {
		t.Fatal(err)
	}
	pg, err := client.Get(ctx, "foobar", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(pg.Object, "recovered", "spec", "activeBranch"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Update(ctx, pg, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	result, err := ActivateBranch(ctx, ActivatePostgresBranchInput{Postgres: "foobar", Branch: "recovered", TeamSlug: team, EnvironmentName: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Postgres.DesiredActiveBranch == nil || *result.Postgres.DesiredActiveBranch != "recovered" || result.Postgres.ActiveBranch == nil || *result.Postgres.ActiveBranch != "main" {
		t.Errorf("activation result = %+v; wanted desired recovered and observed main", result.Postgres)
	}
}

func TestCreateBranchNeverTreatsDifferentRecoveryAsSameRequest(t *testing.T) {
	input := CreatePostgresBranchInput{Postgres: "orders", Branch: "restore", SourceBranch: "main", TargetTime: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	obj := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"postgres": "orders", "branchName": "restore", "bootstrap": map[string]any{"recovery": map[string]any{"sourceBranch": "main", "targetTime": "2026-09-01T12:00:00Z"}}}}}
	if !sameRecovery(obj, input) {
		t.Fatal("matching recovery should be idempotent")
	}
	input.TargetTime = input.TargetTime.Add(time.Second)
	if sameRecovery(obj, input) {
		t.Fatal("different target time must not be treated as a retry")
	}
	input.TargetTime = input.TargetTime.Add(-time.Second)
	input.SourceBranch = "other"
	if sameRecovery(obj, input) {
		t.Fatal("different source must not be treated as a retry")
	}
}

func TestCreateBranchRejectsUnavailableRecovery(t *testing.T) {
	earliest := time.Date(2026, time.September, 29, 0, 0, 0, 0, time.UTC)
	for _, scenario := range []struct {
		name           string
		missingWindow  bool
		deletingBranch bool
		targetTime     time.Time
		want           string
	}{
		{name: "missing recovery window", missingWindow: true, targetTime: earliest, want: "no recoverable restore window yet"},
		{name: "before recovery window", targetTime: earliest.Add(-time.Second), want: "restore time must not be before the earliest recoverable time 2026-09-29T00:00:00Z"},
		{name: "matching branch being deleted", deletingBranch: true, targetTime: earliest, want: `PostgresBranch "restore" is being deleted`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			scheme, err := kubernetes.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			log, _ := test.NewNullLogger()
			mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(os.DirFS("../../../integration_tests/k8s_resources/postgres_crud"))))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(mgr.Stop)
			ctx := context.Background()
			branchWatcher := NewPostgresBranchWatcher(ctx, mgr)
			postgresWatcher := NewPostgresWatcher(ctx, mgr)
			wait, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if !mgr.WaitForReady(wait) {
				t.Fatal("watchers did not synchronize")
			}
			ctx = NewLoaderContext(ctx, branchWatcher, postgresWatcher, "", "", "nav", mgr.GetDynamicClients())
			input := CreatePostgresBranchInput{
				Postgres: "existing", Branch: "restore", SourceBranch: "main", TargetTime: scenario.targetTime,
				EnvironmentName: "dev", TeamSlug: slug.Slug("pg-crud-team"),
			}
			if scenario.missingWindow {
				client, err := branchWatcher.SystemAuthenticatedClient(ctx, "dev", watcher.WithImpersonatedClientGVR(barmanObjectStoreGVR))
				if err != nil {
					t.Fatal(err)
				}
				stores := client.Namespace(input.TeamSlug.String())
				store, err := stores.Get(ctx, "existing-main-backups", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				unstructured.RemoveNestedField(store.Object, "status", "serverRecoveryWindow")
				if _, err := stores.Update(ctx, store, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			branches, err := branchClient(ctx, input.EnvironmentName, input.TeamSlug)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.deletingBranch {
				branch, err := getBranchResource(ctx, branches, input.Postgres, input.SourceBranch)
				if err != nil {
					t.Fatal(err)
				}
				branch = branch.DeepCopy()
				branch.SetName(nais_io_v1.PostgresBranchObjectName(input.Postgres, input.Branch))
				branch.SetResourceVersion("")
				if err := unstructured.SetNestedField(branch.Object, input.Branch, "spec", "branchName"); err != nil {
					t.Fatal(err)
				}
				if err := unstructured.SetNestedMap(branch.Object, map[string]any{
					"sourceBranch": input.SourceBranch, "targetTime": input.TargetTime.Format(time.RFC3339),
				}, "spec", "bootstrap", "recovery"); err != nil {
					t.Fatal(err)
				}
				deletingAt := metav1.NewTime(time.Now())
				branch.SetDeletionTimestamp(&deletingAt)
				if !sameRecovery(branch, input) {
					t.Fatal("terminating branch must have matching recovery settings")
				}
				if _, err := branches.Create(ctx, branch, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			payload, err := CreateBranch(ctx, input)
			if err == nil || !strings.Contains(err.Error(), scenario.want) || payload != nil {
				t.Fatalf("CreateBranch = (%+v, %v), want nil payload and error containing %q", payload, err, scenario.want)
			}
			if !scenario.deletingBranch {
				if _, err := branches.Get(ctx, nais_io_v1.PostgresBranchObjectName(input.Postgres, input.Branch), metav1.GetOptions{}); err == nil {
					t.Fatal("rejected request created a branch")
				}
			}
		})
	}
}

func TestFirstRecoverabilityPoint(t *testing.T) {
	want := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	objectStore := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"serverRecoveryWindow": map[string]any{
				"pg-orders-main": map[string]any{
					"firstRecoverabilityPoint": want.Format(time.RFC3339),
				},
			},
		},
	}}
	got, err := firstRecoverabilityPoint(objectStore, "pg-orders-main")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.Equal(want) {
		t.Fatalf("firstRecoverabilityPoint = %v, want %v", got, want)
	}

	got, err = firstRecoverabilityPoint(objectStore, "missing-cluster")
	if err != nil || got != nil {
		t.Errorf("missing recovery window = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestBarmanObjectStoreName(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"plugins": []any{
			map[string]any{"name": "barman-cloud.cloudnative-pg.io", "parameters": map[string]any{"barmanObjectName": "orders-store"}},
		}},
	}}
	got, err := barmanObjectStoreName(cluster)
	if err != nil || got != "orders-store" {
		t.Fatalf("barmanObjectStoreName = (%q, %v), want (%q, nil)", got, err, "orders-store")
	}
}
