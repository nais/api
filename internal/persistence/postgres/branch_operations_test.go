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
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("watchers did not synchronize")
	}
	ctx = NewLoaderContext(ctx, branchWatcher, "", "", "nav", mgr.GetDynamicClients())
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
