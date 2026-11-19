package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	liberatorv1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"github.com/sirupsen/logrus/hooks/test"
)

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
	postgresWatcher := NewPostgresWatcher(ctx, mgr)
	appWatcher := application.NewWatcher(ctx, mgr)
	jobWatcher := job.NewWatcher(ctx, mgr)
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("watchers did not synchronize")
	}
	ctx = NewLoaderContext(ctx, postgresWatcher, "", "", "nav", mgr.GetDynamicClients())
	ctx = application.NewLoaderContext(ctx, appWatcher, nil, nil, log)
	ctx = job.NewLoaderContext(ctx, jobWatcher, nil)
	team := slug.Slug("postgres-workload-team")
	apps := application.ListAllForTeamInEnvironment(ctx, team, "dev")
	if len(apps) != 1 || apps[0].Spec.Uses == nil {
		t.Fatalf("application uses not loaded: %+v", apps)
	}
	instances, err := ListPostgresForWorkload(ctx, team, "dev", apps[0].Spec.Uses.Postgres)
	if err != nil || len(instances) != 3 || instances[0].Name != "archive" || instances[1].Name != "orders" || instances[2].Name != "reports" {
		t.Fatalf("referenced Postgres = %+v, error = %v", instances, err)
	}
	jobs := job.ListAllForTeamInEnvironment(ctx, team, "dev")
	if len(jobs) != 1 || jobs[0].Spec.Uses == nil {
		t.Fatalf("job uses not loaded: %+v", jobs)
	}
	instances, err = ListPostgresForWorkload(ctx, team, "dev", jobs[0].Spec.Uses.Postgres)
	if err != nil || len(instances) != 3 || instances[0].Name != "archive" || instances[1].Name != "orders" || instances[2].Name != "reports" {
		t.Fatalf("job referenced Postgres = %+v, error = %v", instances, err)
	}
	page, err := pagination.ParsePage(new(3), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, uses := range [][]liberatorv1.PostgresUse{apps[0].Spec.Uses.Postgres, jobs[0].Spec.Uses.Postgres} {
		usages, totalCount, err := WorkloadPostgresesForWorkload(ctx, team, "dev", uses, page)
		if err != nil || totalCount != 3 || len(usages) != 3 {
			t.Fatalf("Postgres uses = %+v, %d, %v", usages, totalCount, err)
		}
		for i, name := range []string{"archive", "orders", "reports"} {
			if usages[i].Postgres.Name != name {
				t.Errorf("Postgres use %d = %q, want %q", i, usages[i].Postgres.Name, name)
			}
		}
	}

	usesWithMissingOffPage := append([]liberatorv1.PostgresUse(nil), apps[0].Spec.Uses.Postgres...)
	for index := range usesWithMissingOffPage {
		if usesWithMissingOffPage[index].Name == "reports" {
			usesWithMissingOffPage[index].Name = "missing"
		}
	}
	firstPage, err := pagination.ParsePage(new(1), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	pageUses, totalCount, err := WorkloadPostgresesForWorkload(ctx, team, "dev", usesWithMissingOffPage, firstPage)
	if err != nil || totalCount != 3 || len(pageUses) != 1 || pageUses[0].Postgres.Name != "archive" {
		t.Fatalf("first Postgres use page = (%+v, %d), error = %v", pageUses, totalCount, err)
	}
	reportsPage, err := pagination.ParsePage(new(3), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := WorkloadPostgresesForWorkload(ctx, team, "dev", usesWithMissingOffPage, reportsPage); err == nil {
		t.Fatal("expected an error for a missing Postgres on the requested page")
	}

	for _, name := range []string{"orders", "reports", "archive"} {
		pg, err := GetPostgres(ctx, team, "dev", name)
		if err != nil {
			t.Fatal(err)
		}
		workloads := WorkloadsForPostgres(ctx, pg)
		if len(workloads) != 2 || workloads[0].Workload.GetName() != "consumer" || workloads[1].Workload.GetName() != "scheduled-reader" {
			t.Errorf("aggregate workloads for %q = %+v", pg.Name, workloads)
		}
		for _, current := range workloads {
			if current.Postgres != pg {
				t.Error("workload did not retain the resolved parent; nested Postgres fields must not trigger another lookup")
			}
		}
	}
	if workloads := WorkloadsForPostgres(ctx, &Postgres{Name: "unreferenced", TeamSlug: team, EnvironmentName: "dev"}); len(workloads) != 0 {
		t.Errorf("unreferenced Postgres workloads = %+v", workloads)
	}
	for _, pg := range []*Postgres{
		{Name: "orders", TeamSlug: team, EnvironmentName: "prod"},
		{Name: "orders", TeamSlug: slug.Slug("another-team"), EnvironmentName: "dev"},
	} {
		if workloads := WorkloadsForPostgres(ctx, pg); len(workloads) != 0 {
			t.Errorf("workloads leaked across team or environment for %+v: %+v", pg, workloads)
		}
	}
}

func TestReadyPostgresUsesObservedCluster(t *testing.T) {
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
	postgresWatcher := NewPostgresWatcher(ctx, mgr)
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("Postgres watcher did not synchronize")
	}
	ctx = NewLoaderContext(ctx, postgresWatcher, "", "", "nav", mgr.GetDynamicClients())
	team := slug.Slug("someteamname")
	state, err := readyPostgresState(ctx, team, "dev", "foobar")
	if err != nil || state != PostgresStateAvailable {
		t.Errorf("ready database = %q, %v", state, err)
	}
	state, err = readyPostgresState(ctx, team, "dev", "progressing")
	if err != nil || state == PostgresStateAvailable {
		t.Errorf("provisioning database = %q, %v", state, err)
	}
	if _, err := readyPostgresState(ctx, team, "dev", "missing"); err == nil {
		t.Error("accepted missing database")
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
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(wait) {
		t.Fatal("Postgres watcher did not synchronize")
	}
	postgresWatcher := NewPostgresWatcher(ctx, mgr)
	ctx = NewLoaderContext(ctx, postgresWatcher, "", "", "nav", mgr.GetDynamicClients())
	input := CreatePostgresAccessInput{
		Postgres: "foobar", TeamSlug: slug.Slug("myteam"),
		EnvironmentName: "dev", AccessLevel: PostgresAccessLevelRead,
		Reason: "Investigating missing instance",
	}
	err = input.Validate(ctx)
	if err == nil || !strings.Contains(err.Error(), `Could not find Postgres "foobar"`) {
		t.Errorf("validation error = %v, want named missing instance", err)
	}
}
