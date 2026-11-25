package v0s

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type overviewClient struct {
	protoapi.VulnerabilityServiceClient
	list func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error)
}

func (c *overviewClient) ListWorkloads(_ context.Context, req *protoapi.ListWorkloadsRequest, _ ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error) {
	return c.list(req)
}

func overviewLoaders(t *testing.T, c *overviewClient, names ...string) *loaders {
	t.Helper()
	ctx := WithManager(t.Context(), &Manager{Vulnerabilities: c})
	ctx = NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		env := &inventoryapi.EnvironmentWorkloadImages{Environment: "dev"}
		for _, name := range names {
			env.Workloads = append(env.Workloads, &inventoryapi.WorkloadImage{
				TeamSlug: "team", Name: name, Image: "repo/" + name + ":2",
			})
		}
		return &inventoryapi.ListWorkloadImagesResponse{Environments: []*inventoryapi.EnvironmentWorkloadImages{env}}, nil
	})
	l, err := fromContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestWorkloadOverviewBatchPreservesMissingAndStaleWorkloads(t *testing.T) {
	calls := 0
	c := &overviewClient{list: func(req *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
		calls++
		if req.Overview == nil || req.Overview.GetFindingFilter().GetPackageType() != "npm" {
			t.Errorf("unexpected overview options: %v", req.Overview)
		}
		if req.WorkloadSelector == nil || len(req.WorkloadSelector.Workloads) != 5 {
			t.Errorf("expected one explicit batch of five workloads: %v", req.WorkloadSelector)
		}
		row := func(name string) *protoapi.Workload {
			return &protoapi.Workload{
				Environment: "dev", Team: "team", Name: name, ImageReference: "repo/" + name + ":2",
				Image:         &protoapi.Image{Name: "repo/" + name, Digest: "sha256:" + name, Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED},
				FindingCounts: &protoapi.WorkloadFindingCounts{SeverityCounts: &protoapi.VulnerabilitySeverityCounts{}},
			}
		}
		app, stale, clean, unresolved := row("app"), row("stale"), row("clean"), row("unresolved")
		app.FindingCounts = &protoapi.WorkloadFindingCounts{
			FindingCount: 7, UniqueVulnerabilityCount: 3, UniqueKevVulnerabilityCount: 2,
			SeverityCounts: &protoapi.VulnerabilitySeverityCounts{Critical: 1, High: 2},
		}
		stale.ImageReference = "repo/stale:1"
		stale.FindingCounts = app.FindingCounts
		unresolved.Image, unresolved.ResolveError = nil, "cannot resolve"
		return &protoapi.ListWorkloadsResponse{Workloads: []*protoapi.Workload{app, stale, clean, unresolved}, TotalCount: new(int64(4))}, nil
	}}
	l := overviewLoaders(t, c, "app", "stale", "fresh", "clean", "unresolved")
	filter, err := proto.Marshal(&protoapi.FindingFilter{PackageType: "npm"})
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]workloadOverviewKey, 0, 5)
	for _, name := range []string{"app", "stale", "fresh", "clean", "unresolved"} {
		keys = append(keys, workloadOverviewKey{workloadIdentity: workloadIdentity{workload: workloadKey{"dev", "team", name}}, image: "repo/" + name + ":2", filter: string(filter)})
	}
	values, err := l.workloadOverviews.LoadAll(t.Context(), keys)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("batch used %d backend calls, want 1", calls)
	}
	if values[0].FindingCount != 7 || values[0].UniqueVulnerabilityCount != 3 || values[0].KnownExploitedVulnerabilityCount != 2 {
		t.Errorf("server counts were not preserved: %+v", values[0])
	}
	if diff := cmp.Diff(&VulnerabilitySeverityCounts{Critical: 1, High: 2}, values[0].CountsBySeverity); diff != "" {
		t.Errorf("severity counts (-want +got):\n%s", diff)
	}
	for _, i := range []int{1, 2} {
		want := &VulnerabilityWorkloadOverview{
			CountsBySeverity: &VulnerabilitySeverityCounts{},
			Coverage:         &VulnerabilityCoverage{Status: VulnerabilityCoverageStatusPending},
		}
		if diff := cmp.Diff(want, values[i]); diff != "" {
			t.Errorf("%s must remain pending without obsolete counts (-want +got):\n%s", keys[i].workload.name, diff)
		}
	}
	if values[3].Coverage.Status != VulnerabilityCoverageStatusScanned || values[3].FindingCount != 0 {
		t.Errorf("clean scanned workload: %+v", values[3])
	}
	if values[4].Coverage.Status != VulnerabilityCoverageStatusUnresolvedImage || values[4].Coverage.Error == nil {
		t.Errorf("unresolved workload: %+v", values[4])
	}
	if _, err := l.workloadOverviews.Load(t.Context(), keys[0]); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Error("repeated overview bypassed request cache")
	}
}

func TestWorkloadOverviewDoesNotHideBackendFailures(t *testing.T) {
	for _, tt := range []struct {
		name string
		list func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error)
	}{
		{name: "unavailable", list: func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
			return nil, status.Error(codes.Unavailable, "KEV catalog unavailable")
		}},
		{name: "legacy response", list: func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
			return &protoapi.ListWorkloadsResponse{}, nil
		}},
		{name: "missing counts", list: func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
			return &protoapi.ListWorkloadsResponse{TotalCount: new(int64(1)), Workloads: []*protoapi.Workload{{
				Environment: "dev", Team: "team", Name: "app", ImageReference: "repo/app:2",
			}}}, nil
		}},
		{name: "stalled pagination", list: func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
			return &protoapi.ListWorkloadsResponse{TotalCount: new(int64(0)), NextPageToken: "same"}, nil
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			l := overviewLoaders(t, &overviewClient{list: tt.list}, "app")
			value, err := l.workloadOverviews.Load(t.Context(), workloadOverviewKey{workloadIdentity: workloadIdentity{workload: workloadKey{"dev", "team", "app"}}, image: "repo/app:2"})
			if err == nil || value != nil {
				t.Fatalf("backend failure became a successful zero count: %+v, %v", value, err)
			}
		})
	}
}

func TestWorkloadOverviewBatchFollowsContinuationWithoutRowCursors(t *testing.T) {
	calls := 0
	c := &overviewClient{list: func(req *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
		calls++
		if req.WorkloadSelector == nil || len(req.WorkloadSelector.Workloads) != 2 || req.Overview == nil {
			t.Fatalf("batch selection was not preserved: %v", req)
		}
		name, count, next := "app", int64(3), "continuation"
		switch req.PageToken {
		case "":
		case "continuation":
			name, count, next = "job", 1, ""
		default:
			t.Fatalf("unexpected continuation token %q", req.PageToken)
		}
		return &protoapi.ListWorkloadsResponse{
			TotalCount: new(int64(2)), NextPageToken: next,
			Workloads: []*protoapi.Workload{{
				Environment: "dev", Team: "team", Name: name, ImageReference: "repo/" + name + ":2",
				Image: &protoapi.Image{Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED},
				FindingCounts: &protoapi.WorkloadFindingCounts{
					FindingCount: count, UniqueVulnerabilityCount: 1,
					SeverityCounts: &protoapi.VulnerabilitySeverityCounts{High: 1},
				},
			}},
		}, nil
	}}
	l := overviewLoaders(t, c, "app", "job")
	values, err := l.workloadOverviews.LoadAll(t.Context(), []workloadOverviewKey{
		{workloadIdentity: workloadIdentity{workload: workloadKey{"dev", "team", "app"}}, image: "repo/app:2"},
		{workloadIdentity: workloadIdentity{workload: workloadKey{"dev", "team", "job"}}, image: "repo/job:2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || values[0].FindingCount != 3 || values[1].FindingCount != 1 {
		t.Fatalf("batch was truncated: calls=%d, values=%+v", calls, values)
	}
}
