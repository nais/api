package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/sirupsen/logrus/hooks/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDeletePostgresBranchRequiresInactiveBranch(t *testing.T) {
	tests := []struct {
		name       string
		postgres   map[string]any
		wantDenied bool
	}{
		{"selected in status", map[string]any{"status": map[string]any{"activeBranch": "restored"}}, true},
		{"selected in spec", map[string]any{"spec": map[string]any{"activeBranch": "restored"}}, true},
		{"pending switch", map[string]any{"spec": map[string]any{"activeBranch": "restored"}, "status": map[string]any{"activeBranch": "original"}}, true},
		{"inactive", map[string]any{"status": map[string]any{"activeBranch": "original"}}, false},
		{"default active", map[string]any{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			postgres := &unstructured.Unstructured{Object: tt.postgres}
			postgres.SetName("orders")
			requested := "restored"
			if tt.name == "default active" {
				requested = "main"
			}
			err := ensureInstanceMayBeDeleted(requested, postgres)
			if (err != nil) != tt.wantDenied {
				t.Errorf("deletion error = %v; denied = %v", err, tt.wantDenied)
			}
		})
	}
}

func TestWorkloadUsesMultiplePostgresResources(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := test.NewNullLogger()
	mgr, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(os.DirFS("../../../integration_tests/k8s_resources/postgres_workloads"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	ctx := context.Background()
	postgresBranchWatcher := NewPostgresBranchWatcher(ctx, mgr)
	appWatcher := application.NewWatcher(ctx, mgr)
	jobWatcher := job.NewWatcher(ctx, mgr)
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("watchers did not synchronize")
	}
	ctx = NewLoaderContext(ctx, postgresBranchWatcher, "", "", "nav", mgr.GetDynamicClients())
	ctx = application.NewLoaderContext(ctx, appWatcher, nil, log)
	ctx = job.NewLoaderContext(ctx, jobWatcher, nil)
	team := slug.Slug("postgres-workload-team")
	apps := application.ListAllForTeamInEnvironment(ctx, team, "dev")
	if len(apps) != 1 || apps[0].Spec.Uses == nil {
		t.Fatalf("application uses not loaded: %+v", apps)
	}
	instances, err := ListForWorkload(ctx, team, "dev", apps[0].Spec.Uses.Postgres)
	if err != nil || len(instances) != 2 || instances[0].Name != "green" || instances[1].Name != "recovered" {
		t.Fatalf("selected instances = %+v, error = %v", instances, err)
	}
	jobs := job.ListAllForTeamInEnvironment(ctx, team, "dev")
	if len(jobs) != 1 || jobs[0].Spec.Uses == nil {
		t.Fatalf("job uses not loaded: %+v", jobs)
	}
	instances, err = ListForWorkload(ctx, team, "dev", jobs[0].Spec.Uses.Postgres)
	if err != nil || len(instances) != 2 || instances[0].Name != "green" || instances[1].Name != "recovered" {
		t.Fatalf("job selected instances = %+v, error = %v", instances, err)
	}
	for _, entry := range []struct{ postgres, branch string }{{"orders", "green"}, {"reports", "recovered"}} {
		workloads := WorkloadsForInstance(ctx, team, "dev", entry.postgres, entry.branch)
		if len(workloads) != 2 || workloads[0].GetName() != "consumer" || workloads[1].GetName() != "scheduled-reader" {
			t.Errorf("workloads for %q = %+v", entry.branch, workloads)
		}
	}
}

func TestReadyPostgresBranchUsesConcreteName(t *testing.T) {
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
	postgresBranchWatcher := NewPostgresBranchWatcher(ctx, mgr)
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("PostgresBranch watcher did not synchronize")
	}
	ctx = NewLoaderContext(ctx, postgresBranchWatcher, "", "", "nav", mgr.GetDynamicClients())
	team := slug.Slug("someteamname")
	for _, name := range []string{"main", "recovered"} {
		instance, err := GetReadyPostgresBranch(ctx, team, "dev", "foobar", name)
		if err != nil || instance.State != PostgresBranchStateAvailable || instance.Name != name || instance.PostgresName != "foobar" {
			t.Errorf("%s: got instance %+v, error %v", name, instance, err)
		}
	}
	instance, err := GetReadyPostgresBranch(ctx, team, "dev", "progressing", "main")
	if err != nil || instance.State == PostgresBranchStateAvailable {
		t.Errorf("progressing instance = %+v, error %v", instance, err)
	}
	_, err = GetReadyPostgresBranch(ctx, team, "dev", "foobar", "missing")
	if !errors.Is(err, &watcher.ErrorNotFound{}) || strings.Contains(err.Error(), "foobar-missing-") {
		t.Errorf("missing instance error = %v; want local branch name only", err)
	}
	_, err = GetPostgresBranch(ctx, team, "dev", "progressing", "recovered")
	if !errors.Is(err, &watcher.ErrorNotFound{}) {
		t.Errorf("branch in wrong Postgres error = %v", err)
	}
	pg, err := GetPostgres(ctx, team, "dev", "foobar")
	if err != nil {
		t.Fatal(err)
	}
	branches := ListForPostgres(ctx, pg, nil, nil)
	nodes := branches.Nodes()
	if len(nodes) != 2 || nodes[0].Name != "main" || nodes[1].Name != "recovered" {
		t.Errorf("Postgres branches = %+v", nodes)
	}
}

func TestCreatePostgresAccessRejectsMissingInstance(t *testing.T) {
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
	ctx := context.Background()
	postgresBranchWatcher := NewPostgresBranchWatcher(ctx, mgr)
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("PostgresBranch watcher did not synchronize")
	}
	ctx = NewLoaderContext(ctx, postgresBranchWatcher, "", "", "nav", mgr.GetDynamicClients())
	input := CreatePostgresAccessInput{
		Postgres: "foobar", Branch: "missing", TeamSlug: slug.Slug("myteam"),
		EnvironmentName: "dev", AccessLevel: PostgresAccessLevelRead,
		Reason: "Investigating missing instance",
	}
	err = input.Validate(ctx)
	if err == nil || !strings.Contains(err.Error(), `Could not find PostgresBranch "missing"`) {
		t.Errorf("validation error = %v, want named missing instance", err)
	}
}
