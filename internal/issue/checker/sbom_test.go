package checker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type sbomClient func(context.Context, *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error)

func (f sbomClient) ListWorkloads(ctx context.Context, req *protoapi.ListWorkloadsRequest, _ ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error) {
	return f(ctx, req)
}

func sbomWorkload(status protoapi.ImageStatus) *protoapi.Workload {
	return &protoapi.Workload{
		Environment: "dev", Team: "team", Name: "app", ImageReference: "registry/app@sha256:current",
		Image: &protoapi.Image{Status: status},
	}
}

func TestV0sSBOMCoverage(t *testing.T) {
	key := sbomKey{"dev", "team", "app", issue.ResourceTypeApplication}
	for _, tt := range []struct {
		name string
		row  *protoapi.Workload
		want sbomResult
	}{
		{"missing", sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM), sbomResult{key: true}},
		{"scanned", sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_SCANNED), sbomResult{key: false}},
		{"pending", sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_PENDING), sbomResult{}},
		{"failed", sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_FAILED), sbomResult{}},
		{"unspecified", sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_UNSPECIFIED), sbomResult{}},
		{"unknown status", sbomWorkload(protoapi.ImageStatus(100)), sbomResult{}},
		{"missing workload", nil, sbomResult{}},
		{"unresolved", &protoapi.Workload{Environment: "dev", Team: "team", Name: "app", ResolveError: "not found"}, sbomResult{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := sbomClient(func(_ context.Context, req *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
				if req.Overview != nil || len(req.GetWorkloadSelector().GetWorkloads()) != 1 {
					t.Fatalf("want explicit coverage-only selection, got %v", req)
				}
				response := &protoapi.ListWorkloadsResponse{}
				if tt.row != nil {
					response.Workloads = []*protoapi.Workload{tt.row}
				}
				return response, nil
			})
			got, err := v0sSBOM(t.Context(), client, sbomInventory{key: "registry/app@sha256:current"})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("coverage (-want +got):\n%s", diff)
			}
		})
	}
	for _, name := range []string{"stale image", "missing image", "resolve error", "empty current image"} {
		t.Run(name, func(t *testing.T) {
			row := sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM)
			image := row.ImageReference
			switch name {
			case "stale image":
				row.ImageReference = "old-image"
			case "missing image":
				row.Image = nil
			case "resolve error":
				row.ResolveError = "unresolved"
			case "empty current image":
				image = ""
			}
			got, err := v0sSBOM(t.Context(), sbomClient(func(context.Context, *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
				return &protoapi.ListWorkloadsResponse{Workloads: []*protoapi.Workload{row}}, nil
			}), sbomInventory{key: image})
			if err != nil || len(got) != 0 {
				t.Fatalf("unresolved must not become NO_SBOM: result=%v, error=%v", got, err)
			}
		})
	}
}

func TestV0sSBOMPagination(t *testing.T) {
	key := sbomKey{"dev", "team", "app", issue.ResourceTypeJob}
	for _, mode := range []string{"complete", "later error", "nil response", "stalled", "cycle", "conflict", "outside selection", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			client := sbomClient(func(ctx context.Context, req *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
				calls++
				if mode == "timeout" {
					cancel()
					<-ctx.Done()
					return nil, ctx.Err()
				}
				if calls == 1 {
					return &protoapi.ListWorkloadsResponse{Workloads: []*protoapi.Workload{sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM)}, NextPageToken: "second"}, nil
				}
				switch mode {
				case "later error":
					return nil, errors.New("backend unavailable")
				case "nil response":
					return nil, nil
				case "stalled":
					return &protoapi.ListWorkloadsResponse{NextPageToken: req.PageToken}, nil
				case "cycle":
					if calls == 2 {
						return &protoapi.ListWorkloadsResponse{NextPageToken: "third"}, nil
					}
					return &protoapi.ListWorkloadsResponse{NextPageToken: "second"}, nil
				case "conflict":
					return &protoapi.ListWorkloadsResponse{Workloads: []*protoapi.Workload{sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_SCANNED)}}, nil
				case "outside selection":
					row := sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM)
					row.Name = "other"
					return &protoapi.ListWorkloadsResponse{Workloads: []*protoapi.Workload{row}}, nil
				}
				return &protoapi.ListWorkloadsResponse{Workloads: []*protoapi.Workload{sbomWorkload(protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM)}}, nil
			})
			got, err := v0sSBOM(ctx, client, sbomInventory{key: "registry/app@sha256:current"})
			if mode == "complete" {
				if err != nil || len(got) != 1 || !got[key] {
					t.Fatalf("complete deduplicated job result=%v, error=%v", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatalf("incomplete collection must fail without partial results: %v, %v", got, err)
			}
		})
	}
}

func TestSBOMSourceErrorAndEmptyInventory(t *testing.T) {
	key := sbomKey{"dev", "team", "app", issue.ResourceTypeApplication}
	calls := 0
	sourceErr := errors.New("v0s failed")
	check := sbomCheck{
		v0s: sbomClient(func(context.Context, *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
			calls++
			return nil, sourceErr
		}),
	}
	if _, err := check.collect(t.Context(), sbomInventory{key: "image"}); !errors.Is(err, sourceErr) {
		t.Fatalf("expected source error, got %v", err)
	}
	empty, err := check.collect(t.Context(), sbomInventory{})
	if err != nil || len(empty) != 0 || calls != 1 {
		t.Fatalf("empty inventory must not call a source: %v %v, calls=%d", empty, err, calls)
	}
}

func TestV0sSBOMBatchSelection(t *testing.T) {
	inventory := make(sbomInventory)
	for i := range 251 {
		inventory[sbomKey{"dev", "team", fmt.Sprintf("app-%03d", i), issue.ResourceTypeApplication}] = "image"
	}
	calls := 0
	result, err := v0sSBOM(t.Context(), sbomClient(func(_ context.Context, req *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
		calls++
		response := &protoapi.ListWorkloadsResponse{}
		for _, ref := range req.GetWorkloadSelector().GetWorkloads() {
			response.Workloads = append(response.Workloads, &protoapi.Workload{
				Environment: ref.Environment, Team: ref.Team, Name: ref.Name,
				ImageReference: "image", Image: &protoapi.Image{Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED},
			})
		}
		return proto.Clone(response).(*protoapi.ListWorkloadsResponse), nil
	}), inventory)
	if err != nil || len(result) != 251 || calls != 2 {
		t.Fatalf("result size=%d calls=%d error=%v", len(result), calls, err)
	}
}

func TestSBOMIssuesCurrentImageAndErrors(t *testing.T) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clusters, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev-gcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logrustest.NewNullLogger()
	manager, err := watcher.NewManager(scheme, clusters, log, watcher.WithClientCreator(fake.Clients(os.DirFS("testdata"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Stop)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	apps := application.NewWatcher(ctx, manager)
	jobs := job.NewWatcher(ctx, manager)
	wait, cancelWait := context.WithTimeout(ctx, 5*time.Second)
	defer cancelWait()
	if !manager.WaitForReady(wait) {
		t.Fatal("inventory did not synchronize")
	}
	backendFailed := false
	check := sbomCheck{
		workloads: Workload{AppWatcher: *apps, JobWatcher: *jobs, log: log},
		v0s: sbomClient(func(context.Context, *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
			if backendFailed {
				return nil, errors.New("source failed")
			}
			return &protoapi.ListWorkloadsResponse{Workloads: []*protoapi.Workload{
				{
					Environment: "dev-gcp", Team: "devteam", Name: "ext-app", ImageReference: "ghcr.io/nais/ext-app:latest",
					Image: &protoapi.Image{Status: protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM},
				},
				{
					Environment: "dev-gcp", Team: "devteam", Name: "internal-only", ImageReference: "old-image",
					Image: &protoapi.Image{Status: protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM},
				},
			}}, nil
		}),
	}
	rows, err := check.issues(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []Issue{{
		IssueType: issue.IssueTypeMissingSBOM, ResourceName: "ext-app", ResourceType: issue.ResourceTypeApplication,
		Team: "devteam", Env: "dev-gcp", Severity: issue.SeverityWarning,
		Message: "Image 'ghcr.io/nais/ext-app:latest' is missing a Software Bill of Materials (SBOM)",
	}}
	if diff := cmp.Diff(want, rows); diff != "" {
		t.Errorf("issues (-want +got):\n%s", diff)
	}
	backendFailed = true
	rows, err = check.issues(ctx)
	if err == nil || len(rows) != 0 {
		t.Fatalf("failed collection must return an error and no issue rows: %v, %v", rows, err)
	}
}

func TestSBOMRequiresEndpoint(t *testing.T) {
	log, _ := logrustest.NewNullLogger()
	_, err := New(Config{}, nil, nil, log)
	if err == nil {
		t.Fatal("missing v0s endpoint must fail")
	}
}
