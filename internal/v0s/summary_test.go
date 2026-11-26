package v0s_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/workload"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type selectedWorkload struct {
	workload.Workload
	name  string
	kind  workload.Type
	image string
}

func (w selectedWorkload) GetName() string            { return w.name }
func (w selectedWorkload) GetEnvironmentName() string { return "dev" }
func (w selectedWorkload) GetTeamSlug() slug.Slug     { return slug.Slug("team-one") }
func (w selectedWorkload) GetType() workload.Type     { return w.kind }
func (w selectedWorkload) GetImageString() string {
	if w.image != "" {
		return w.image
	}
	return "repo/" + w.name + ":1"
}

func TestSummaryCoverageCountsSameNameWorkloadTypesAndOwnImages(t *testing.T) {
	for _, scan := range []string{"missing", "scanned", "failed"} {
		t.Run(scan, func(t *testing.T) {
			c := &client{list: func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
				return &protoapi.ListFindingsResponse{}
			}}
			ctx := c.context(t, "same")
			backendCoverage := &protoapi.WorkloadCoverage{}
			want := &workload.VulnerabilityCoverageCounts{TotalWorkloads: 2, Pending: 1}
			switch scan {
			case "missing":
				c.workloads = nil
				want.Pending = 2
			case "scanned":
				c.workloads[0].ImageReference = "repo/job:2"
				c.workloads[0].Image.Status = protoapi.ImageStatus_IMAGE_STATUS_SCANNED
				backendCoverage.TotalWorkloadCount, backendCoverage.ScannedCount = 1, 1
				want.Scanned = 1
			case "failed":
				c.workloads[0].ImageReference = "repo/job:2"
				c.workloads[0].Image.Status = protoapi.ImageStatus_IMAGE_STATUS_FAILED
				backendCoverage.TotalWorkloadCount, backendCoverage.FailedCount = 1, 1
				want.Failed = 1
			}
			c.summary = func(req *protoapi.GetSummaryRequest) (*protoapi.GetSummaryResponse, error) {
				if int64(len(req.WorkloadSelector.GetWorkloads())) != backendCoverage.TotalWorkloadCount {
					t.Errorf("summary selected the wrong workload image: %v", req.WorkloadSelector)
				}
				return &protoapi.GetSummaryResponse{
					SeverityCounts: &protoapi.VulnerabilitySeverityCounts{}, Coverage: backendCoverage,
				}, nil
			}
			app := selectedWorkload{name: "same", kind: workload.TypeApplication, image: "repo/app:2"}
			job := selectedWorkload{name: "same", kind: workload.TypeJob, image: "repo/job:2"}
			page, err := v0s.FindingsForWorkloads(ctx, v0s.Scope{Team: slug.Slug("team-one")},
				[]workload.Workload{app, job, app}, nil, new(1), nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := page.Summary(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got.Coverage); diff != "" {
				t.Fatalf("coverage (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSummaryExplicitSelectionIncludesWorkloadsAbsentFromImageInventory(t *testing.T) {
	c := &client{}
	ctx := c.context(t, "app", "outside", "stale")
	c.workloads[0].Image.Status = protoapi.ImageStatus_IMAGE_STATUS_SCANNED
	c.workloads[2].ImageReference = "repo/stale:old"
	workloadCalls := 0
	c.listWorkloads = func(req *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
		workloadCalls++
		if req.WorkloadSelector == nil {
			t.Fatal("metadata-selected reconciliation fetched unrestricted inventory")
		}
		names := make([]string, 0)
		for _, ref := range req.WorkloadSelector.Workloads {
			if ref.Team != "team-one" || ref.Environment != "dev" {
				t.Errorf("unexpected selected workload: %v", ref)
			}
			names = append(names, ref.Name)
		}
		if diff := cmp.Diff([]string{"app", "stale", "without-image"}, names); diff != "" {
			t.Errorf("reconciliation selector (-want +got):\n%s", diff)
		}
		switch req.PageToken {
		case "":
			return &protoapi.ListWorkloadsResponse{Workloads: c.workloads[:1], NextPageToken: "rest"}, nil
		case "rest":
			return &protoapi.ListWorkloadsResponse{Workloads: c.workloads[2:]}, nil
		default:
			t.Fatalf("unexpected workload cursor: %q", req.PageToken)
			return nil, nil
		}
	}
	c.list = func(req *protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		if refs := req.WorkloadSelector.GetWorkloads(); len(refs) != 1 || refs[0].Name != "app" {
			t.Errorf("findings did not retain the selected current workloads: %v", req.WorkloadSelector)
		}
		return &protoapi.ListFindingsResponse{}
	}
	c.summary = func(req *protoapi.GetSummaryRequest) (*protoapi.GetSummaryResponse, error) {
		if refs := req.WorkloadSelector.GetWorkloads(); len(refs) != 1 || refs[0].Name != "app" {
			t.Errorf("summary selection differs from findings: %v", req.WorkloadSelector)
		}
		if req.FindingFilter.PackageType != "npm" || !req.FindingFilter.HasFix {
			t.Errorf("summary lost finding predicates: %v", req.FindingFilter)
		}
		return &protoapi.GetSummaryResponse{
			FindingCount: 9, UniqueVulnerabilityCount: 2, AffectedWorkloadCount: 1,
			SeverityCounts: &protoapi.VulnerabilitySeverityCounts{Critical: 1, High: 1},
			Coverage:       &protoapi.WorkloadCoverage{TotalWorkloadCount: 1, ScannedCount: 1},
		}, nil
	}
	page, err := v0s.FindingsForWorkloads(ctx, v0s.Scope{},
		[]workload.Workload{selectedWorkload{name: "app"}, selectedWorkload{name: "stale"}, selectedWorkload{name: "without-image"}},
		&v0s.VulnerabilityFindingFilter{PackageType: new("npm"), OnlyWithFix: new(true)}, new(1), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := page.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := &v0s.VulnerabilitySelectionSummary{
		FindingCount: 9, UniqueVulnerabilityCount: 2, AffectedWorkloadCount: 1,
		CountsBySeverity: &v0s.VulnerabilitySeverityCounts{Critical: 1, High: 1},
		Coverage:         &workload.VulnerabilityCoverageCounts{TotalWorkloads: 3, Scanned: 1, Pending: 2},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("selected summary (-want +got):\n%s", diff)
	}
	if _, err := v0s.FindingsForWorkloads(ctx, v0s.Scope{Team: "team-one", Environment: "dev", Workload: "app"},
		[]workload.Workload{selectedWorkload{name: "app"}}, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if workloadCalls != 2 {
		t.Fatalf("selected inventory was not paged once and reused for row reads: %d calls", workloadCalls)
	}
}

func TestSummaryExplicitEmptySelectionIsNotUnrestricted(t *testing.T) {
	c := &client{}
	ctx := c.context(t, "app")
	c.listWorkloads = func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
		t.Fatal("empty selection must not fetch backend workload inventory")
		return nil, nil
	}
	c.list = func(req *protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		if req.WorkloadSelector == nil || len(req.WorkloadSelector.Workloads) != 0 {
			t.Errorf("empty finding selector became unrestricted: %v", req.WorkloadSelector)
		}
		return &protoapi.ListFindingsResponse{}
	}
	c.summary = func(req *protoapi.GetSummaryRequest) (*protoapi.GetSummaryResponse, error) {
		if req.WorkloadSelector == nil || len(req.WorkloadSelector.Workloads) != 0 {
			t.Errorf("empty summary selector became unrestricted: %v", req.WorkloadSelector)
		}
		return &protoapi.GetSummaryResponse{
			SeverityCounts: &protoapi.VulnerabilitySeverityCounts{}, Coverage: &protoapi.WorkloadCoverage{},
		}, nil
	}
	page, err := v0s.FindingsForWorkloads(ctx, v0s.Scope{Team: slug.Slug("team-one")}, nil, nil, new(1), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := page.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.FindingCount != 0 || got.Coverage.TotalWorkloads != 0 {
		t.Fatalf("empty selection counted unselected inventory: %+v", got)
	}
	affected, err := page.AffectedWorkloads(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(affected.Edges) != 0 || affected.PageInfo.TotalCount != 0 {
		t.Fatalf("empty selection returned affected workloads: %+v", affected)
	}
}

func TestSummaryUsesServerCountsAndCompleteInventoryCoverage(t *testing.T) {
	c := &client{list: func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		return &protoapi.ListFindingsResponse{}
	}}
	ctx := c.context(t, "app", "stale", "fresh", "unresolved")
	c.workloads = c.workloads[:3]
	c.workloads[1].ImageReference = "old-image"
	c.workloads[2].Name, c.workloads[2].ImageReference = "unresolved", "repo/unresolved:1"
	c.workloads[2].Image, c.workloads[2].ResolveError = nil, "unresolved"
	c.summary = func(req *protoapi.GetSummaryRequest) (*protoapi.GetSummaryResponse, error) {
		if req.Team != "team-one" || req.Environment != "dev" || req.FindingFilter.Team != "" || req.FindingFilter.Environment != "" {
			t.Errorf("scope must be top-level only: %v", req)
		}
		if req.FindingFilter.PackageType != "npm" || !req.FindingFilter.HasFix {
			t.Errorf("finding filters lost: %v", req.FindingFilter)
		}
		if req.WorkloadSelector == nil || len(req.WorkloadSelector.Workloads) != 2 {
			t.Errorf("selector must retain unresolved but exclude stale/missing workloads: %v", req.WorkloadSelector)
		}
		return &protoapi.GetSummaryResponse{
			FindingCount: 9, UniqueVulnerabilityCount: 3, AffectedWorkloadCount: 1,
			UniqueKevVulnerabilityCount: 2, KevAffectedWorkloadCount: 1,
			SeverityCounts: &protoapi.VulnerabilitySeverityCounts{Critical: 1, High: 2},
			Coverage:       &protoapi.WorkloadCoverage{TotalWorkloadCount: 2, ScannedCount: 1, UnresolvedImageCount: 1},
		}, nil
	}
	page, err := v0s.Findings(ctx, v0s.Scope{Team: slug.Slug("team-one"), Environment: "dev"},
		&v0s.VulnerabilityFindingFilter{PackageType: new("NPM"), OnlyWithFix: new(true)}, new(1), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := page.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := &v0s.VulnerabilitySelectionSummary{
		FindingCount: 9, UniqueVulnerabilityCount: 3, AffectedWorkloadCount: 1,
		KnownExploitedVulnerabilityCount: 2, KnownExploitedWorkloadCount: 1,
		CountsBySeverity: &v0s.VulnerabilitySeverityCounts{Critical: 1, High: 2},
		Coverage:         &workload.VulnerabilityCoverageCounts{TotalWorkloads: 4, Scanned: 1, Pending: 2, UnresolvedImage: 1},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("summary (-want +got):\n%s", diff)
	}
}

func TestSummaryEmptySelectionAndFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		result  *protoapi.GetSummaryResponse
		err     error
		wantErr bool
	}{
		{name: "empty", result: &protoapi.GetSummaryResponse{SeverityCounts: &protoapi.VulnerabilitySeverityCounts{}, Coverage: &protoapi.WorkloadCoverage{}}},
		{name: "unavailable", err: status.Error(codes.Unavailable, "KEV unavailable"), wantErr: true},
		{name: "missing counts", result: &protoapi.GetSummaryResponse{}, wantErr: true},
		{name: "changed inventory", result: &protoapi.GetSummaryResponse{
			SeverityCounts: &protoapi.VulnerabilitySeverityCounts{}, Coverage: &protoapi.WorkloadCoverage{TotalWorkloadCount: 1, ScannedCount: 1},
		}, wantErr: true},
		{name: "inconsistent coverage", result: &protoapi.GetSummaryResponse{
			SeverityCounts: &protoapi.VulnerabilitySeverityCounts{}, Coverage: &protoapi.WorkloadCoverage{PendingCount: 1},
		}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &client{
				list: func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
					return &protoapi.ListFindingsResponse{}
				},
				summary: func(req *protoapi.GetSummaryRequest) (*protoapi.GetSummaryResponse, error) {
					if req.WorkloadSelector == nil || len(req.WorkloadSelector.Workloads) != 0 {
						t.Errorf("empty selection became unrestricted: %v", req.WorkloadSelector)
					}

					return tt.result, tt.err
				},
			}
			ctx := c.context(t)
			page, err := v0s.Findings(ctx, v0s.Scope{}, nil, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := page.Summary(ctx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("summary=%+v err=%v, wantErr=%v", got, err, tt.wantErr)
			}
			if tt.wantErr && got != nil {
				t.Error("summary failure returned successful data")
			}
		})
	}
}

func TestSummaryTenantAndTeamScopes(t *testing.T) {
	c := &client{list: func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		return &protoapi.ListFindingsResponse{}
	}}
	ctx := c.context(t, "app")
	c.workloads = append(c.workloads, &protoapi.Workload{
		Environment: "dev", Team: "team-two", Name: "app", ImageReference: "repo/shared:1",
		Image: c.workloads[0].Image,
	})
	ctx = v0s.NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		return &inventoryapi.ListWorkloadImagesResponse{Environments: []*inventoryapi.EnvironmentWorkloadImages{{
			Environment: "dev", Workloads: []*inventoryapi.WorkloadImage{
				{TeamSlug: "team-one", Name: "app", Image: "repo/app:1"},
				{TeamSlug: "team-two", Name: "app", Image: "repo/shared:1"},
			},
		}}}, nil
	})
	for _, scope := range []v0s.Scope{{}, {Team: slug.Slug("team-two")}} {
		t.Run(scope.Team.String(), func(t *testing.T) {
			count := int64(2)
			if scope.Team != "" {
				count = 1
			}
			c.summary = func(req *protoapi.GetSummaryRequest) (*protoapi.GetSummaryResponse, error) {
				if req.Team != scope.Team.String() || req.FindingFilter.Team != "" ||
					req.WorkloadSelector == nil || int64(len(req.WorkloadSelector.Workloads)) != count {
					t.Errorf("incorrect scope selection: %v", req)
				}
				for _, w := range req.WorkloadSelector.GetWorkloads() {
					if scope.Team != "" && w.Team != scope.Team.String() {
						t.Errorf("team summary included %q", w.Team)
					}
				}
				return &protoapi.GetSummaryResponse{
					FindingCount: 3 * count, UniqueVulnerabilityCount: 1, AffectedWorkloadCount: count,
					SeverityCounts: &protoapi.VulnerabilitySeverityCounts{High: 1},
					Coverage:       &protoapi.WorkloadCoverage{TotalWorkloadCount: count, ScannedCount: count},
				}, nil
			}
			page, err := v0s.Findings(ctx, scope, nil, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := page.Summary(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if got.UniqueVulnerabilityCount != 1 || got.FindingCount != 3*count ||
				got.AffectedWorkloadCount != count || got.Coverage.TotalWorkloads != int(count) {
				t.Fatalf("global uniqueness or scope lost: %+v", got)
			}
		})
	}
}
