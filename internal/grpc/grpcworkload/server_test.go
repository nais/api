package grpcworkload_test

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/grpc/grpcworkload"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
)

func TestListImages(t *testing.T) {
	ctx := context.Background()

	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	// "empty" has no fixtures: it must be reported, with no workloads. The
	// nais-system fixture in dev must not be reported.
	ccm, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev", "prod", "empty"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logrustest.NewNullLogger()
	log.SetLevel(logrus.PanicLevel)
	mgr, err := watcher.NewManager(scheme, ccm, log, watcher.WithClientCreator(fake.Clients(os.DirFS("./testdata"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)

	appWatcher := application.NewWatcher(ctx, mgr)
	jobWatcher := job.NewWatcher(ctx, mgr)
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !mgr.WaitForReady(waitCtx) {
		t.Fatal("timed out waiting for watcher manager")
	}

	server := grpcworkload.NewServer((*watchers.AppWatcher)(appWatcher), (*watchers.JobWatcher)(jobWatcher))
	resp, err := server.ListImages(ctx, &protoapi.ListWorkloadImagesRequest{})
	if err != nil {
		t.Fatal(err)
	}

	type workload struct {
		Team, Name string
		Kind       protoapi.WorkloadKind
		Image      string
	}
	got := map[string][]workload{}
	for _, env := range resp.Environments {
		ws := []workload{}
		for _, w := range env.Workloads {
			ws = append(ws, workload{w.TeamSlug, w.Name, w.Kind, w.Image})
		}
		slices.SortFunc(ws, func(a, b workload) int { return strings.Compare(a.Name, b.Name) })
		got[env.Environment] = ws
	}

	want := map[string][]workload{
		"dev": {
			{"myteam", "batch", protoapi.WorkloadKind_JOB, "europe-north1-docker.pkg.dev/nais-io/batch@sha256:2222222222222222222222222222222222222222222222222222222222222222"},
			// The effective image wins over spec.image.
			{"myteam", "deployed", protoapi.WorkloadKind_APPLICATION, "ghcr.io/nais/deployed:new@sha256:1111111111111111111111111111111111111111111111111111111111111111"},
			{"myteam", "tagged", protoapi.WorkloadKind_APPLICATION, "ghcr.io/nais/tagged:1.0"},
		},
		"prod":  {{"otherteam", "nightly", protoapi.WorkloadKind_JOB, "redis:7.2"}},
		"empty": {},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListImages mismatch (-want +got):\n%s", diff)
	}
}
