package fake

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

type inventory struct {
	inventoryapi.UnimplementedWorkloadsServer
	response *inventoryapi.ListWorkloadImagesResponse
	err      error
}

func (i *inventory) ListImages(context.Context, *inventoryapi.ListWorkloadImagesRequest) (*inventoryapi.ListWorkloadImagesResponse, error) {
	return i.response, i.err
}

func testClient() (*Client, *inventory) {
	i := &inventory{response: &inventoryapi.ListWorkloadImagesResponse{
		Environments: []*inventoryapi.EnvironmentWorkloadImages{{
			Environment: "dev",
			Workloads: []*inventoryapi.WorkloadImage{
				{TeamSlug: "team", Name: "app", Image: "localhost:5000/example/app:latest"},
				{TeamSlug: "other", Name: "job", Image: "localhost:5000/example/app:latest"},
				{TeamSlug: "team", Name: "no-errors", Image: "repo/clean:v1@sha256:clean"},
				{TeamSlug: "team", Name: "app-no-sbom", Image: "repo/no-sbom:v1"},
			},
		}},
	}}
	return NewClient(i), i
}

func TestInventoryOverviewsAndSummary(t *testing.T) {
	c, _ := testClient()
	ctx := t.Context()
	resp, err := c.ListWorkloads(ctx, &protoapi.ListWorkloadsRequest{
		Overview: &protoapi.WorkloadOverviewOptions{FindingFilter: &protoapi.FindingFilter{OnlyKnownExploited: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalCount == nil || *resp.TotalCount != 4 {
		t.Fatalf("overview total count = %v, want 4", resp.TotalCount)
	}
	for _, w := range resp.Workloads {
		want := int64(0)
		if w.Name == "app" || w.Name == "job" {
			want = 1
		}
		if w.FindingCounts == nil || w.FindingCounts.GetFindingCount() != want || w.FindingCounts.GetSeverityCounts() == nil {
			t.Errorf("unexpected overview for %s: %v", w.Name, w.FindingCounts)
		}
		switch w.Name {
		case "app":
			if w.ImageReference != "localhost:5000/example/app:latest" || w.Image.Name != "localhost:5000/example/app" {
				t.Errorf("image reference was not preserved: %v", w)
			}
		case "no-errors":
			if w.Image.Name != "repo/clean" || w.Image.Digest != "sha256:clean" {
				t.Errorf("digest reference was not preserved: %v", w.Image)
			}
		case "app-no-sbom":
			if w.Image.Status != protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM {
				t.Errorf("expected NO_SBOM coverage, got %v", w.Image.Status)
			}
		}
	}
	summary, err := c.GetSummary(ctx, &protoapi.GetSummaryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	want := &protoapi.GetSummaryResponse{
		FindingCount: 10, UniqueVulnerabilityCount: 5, AffectedWorkloadCount: 2,
		UniqueKevVulnerabilityCount: 1, KevAffectedWorkloadCount: 2,
		SeverityCounts: &protoapi.VulnerabilitySeverityCounts{Critical: 1, High: 1, Medium: 1, Low: 1, Unknown: 1},
		Coverage:       &protoapi.WorkloadCoverage{TotalWorkloadCount: 4, ScannedCount: 3, NoSbomCount: 1},
	}
	if diff := cmp.Diff(want, summary, protocmp.Transform()); diff != "" {
		t.Errorf("summary (-want +got):\n%s", diff)
	}
}

func TestFindingSelectionsAndPagination(t *testing.T) {
	c, _ := testClient()
	selectedApp := &protoapi.WorkloadSelector{Workloads: []*protoapi.WorkloadRef{{Environment: "dev", Team: "team", Name: "app"}}}
	for _, tt := range []struct {
		name     string
		filter   *protoapi.FindingFilter
		selector *protoapi.WorkloadSelector
		want     int
	}{
		{name: "all", want: 10},
		{name: "team", filter: &protoapi.FindingFilter{Team: "team"}, want: 5},
		{name: "unknown team", filter: &protoapi.FindingFilter{Team: "missing"}},
		{name: "selected workload", selector: selectedApp, want: 5},
		{name: "empty selection", selector: &protoapi.WorkloadSelector{}},
		{name: "severity", filter: &protoapi.FindingFilter{Severities: []protoapi.Severity{protoapi.Severity_SEVERITY_HIGH}}, want: 2},
		{name: "known exploited", filter: &protoapi.FindingFilter{OnlyKnownExploited: true}, want: 2},
		{name: "fix available", filter: &protoapi.FindingFilter{HasFix: true}, want: 6},
		{name: "package type", filter: &protoapi.FindingFilter{PackageType: "NPM"}, want: 4},
		{name: "alias", filter: &protoapi.FindingFilter{VulnerabilityIds: []string{"FAKE-CVE-2026-1000"}}, want: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := &protoapi.ListFindingsRequest{Filter: tt.filter, WorkloadSelector: tt.selector, PageSize: 1}
			var rows []*protoapi.Finding
			for {
				resp, err := c.ListFindings(t.Context(), req)
				if err != nil {
					t.Fatal(err)
				}
				if len(resp.Findings) != len(resp.Cursors) {
					t.Fatal("missing item cursors")
				}
				rows = append(rows, resp.Findings...)
				if resp.NextPageToken == "" {
					break
				}
				if resp.NextPageToken == req.PageToken || len(rows) > 10 {
					t.Fatal("pagination did not advance")
				}
				req.PageToken = resp.NextPageToken
			}
			if len(rows) != tt.want {
				t.Errorf("got %d findings, want %d", len(rows), tt.want)
			}
			counts, err := c.CountFindings(t.Context(), &protoapi.CountFindingsRequest{
				Filter: tt.filter, WorkloadSelector: tt.selector, GroupBy: protoapi.CountFindingsRequest_GROUP_BY_WORKLOAD,
			})
			if err != nil {
				t.Fatal(err)
			}
			var count int64
			for _, bucket := range counts.Buckets {
				count += bucket.Count
			}
			if count != int64(tt.want) {
				t.Errorf("count = %d, want %d", count, tt.want)
			}
		})
	}
	resp, err := c.ListVulnerabilities(t.Context(), &protoapi.ListVulnerabilitiesRequest{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalCount != 5 || len(resp.Vulnerabilities) != 2 || resp.NextPageToken == "" || len(resp.Cursors) != 2 {
		t.Fatalf("unexpected vulnerability overview page: %v", resp)
	}
	first := resp.Vulnerabilities[0]
	if first.Severity != protoapi.Severity_SEVERITY_CRITICAL || first.FindingCount != 2 || first.AffectedWorkloadCount != 2 {
		t.Errorf("incorrect vulnerability aggregation: %v", first)
	}
	second, err := c.ListVulnerabilities(t.Context(), &protoapi.ListVulnerabilitiesRequest{PageSize: 5, PageToken: resp.Cursors[1]})
	if err != nil {
		t.Fatal(err)
	}
	if second.TotalCount != 5 || len(second.Vulnerabilities) != 3 || second.NextPageToken != "" {
		t.Errorf("unexpected second overview page: %v", second)
	}
	back, err := c.ListVulnerabilities(t.Context(), &protoapi.ListVulnerabilitiesRequest{
		PageSize: 2, PageToken: second.Cursors[0], PageDirection: protoapi.PageDirection_PAGE_DIRECTION_BACKWARD,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(resp.Vulnerabilities, back.Vulnerabilities, protocmp.Transform()); diff != "" {
		t.Errorf("backward overview (-want +got):\n%s", diff)
	}
	if back.TotalCount != 5 || back.GetPageInfo().GetHasPreviousPage() || !back.GetPageInfo().GetHasNextPage() {
		t.Errorf("incorrect backward overview metadata: %v", back)
	}
}

func TestSuppressionLifecycleIsWorkloadScoped(t *testing.T) {
	c, _ := testClient()
	ctx := t.Context()
	key := &protoapi.SuppressionKey{
		Environment: "dev", Workload: "app", ImageName: "localhost:5000/example/app",
		Package: "pkg:golang/example.org/demo@1.0.0", VulnerabilityId: "FAKE-CVE-2026-1000",
	}
	set, err := c.SetSuppressions(ctx, &protoapi.SetSuppressionsRequest{
		Team: "team", Actor: "local-user",
		Suppressions: []*protoapi.NewSuppression{{Key: key, State: protoapi.SuppressionState_SUPPRESSION_STATE_ACCEPTED_RISK, Detail: "Local assessment"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Suppressions) != 1 || set.Suppressions[0].PackageKey != "pkg:golang/example.org/demo" ||
		set.Suppressions[0].VulnerabilityId != "CVE-2026-1000" || !set.Suppressions[0].UpdatedAt.IsValid() {
		t.Fatalf("incorrect suppression: %v", set)
	}
	set.Suppressions[0].Detail = "must not mutate stored state"
	listed, err := c.ListSuppressions(ctx, &protoapi.ListSuppressionsRequest{Team: "team"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Suppressions) != 1 || listed.Suppressions[0].Detail != "Local assessment" {
		t.Fatalf("unexpected stored suppressions: %v", listed)
	}
	empty, err := c.ListSuppressions(ctx, &protoapi.ListSuppressionsRequest{
		Team: "team", PageSize: 1, PageToken: listed.Cursors[0], PageDirection: protoapi.PageDirection_PAGE_DIRECTION_BACKWARD,
	})
	if err != nil || len(empty.GetSuppressions()) != 0 || empty.GetPageInfo().GetHasPreviousPage() || !empty.GetPageInfo().GetHasNextPage() {
		t.Fatalf("empty backward suppression page: %v, %v", empty, err)
	}
	for _, tt := range []struct {
		filter protoapi.SuppressedFilter
		want   int
	}{
		{protoapi.SuppressedFilter_SUPPRESSED_FILTER_EXCLUDE, 9},
		{protoapi.SuppressedFilter_SUPPRESSED_FILTER_INCLUDE, 10},
		{protoapi.SuppressedFilter_SUPPRESSED_FILTER_ONLY, 1},
	} {
		resp, err := c.ListFindings(ctx, &protoapi.ListFindingsRequest{Filter: &protoapi.FindingFilter{Suppressed: tt.filter}})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Findings) != tt.want {
			t.Errorf("%s returned %d findings, want %d", tt.filter, len(resp.Findings), tt.want)
		}
	}
	deleted, err := c.DeleteSuppressions(ctx, &protoapi.DeleteSuppressionsRequest{Team: "team", Keys: []*protoapi.SuppressionKey{key}})
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted.Deleted) != 1 {
		t.Fatalf("expected deleted assessment, got %v", deleted)
	}
	resp, err := c.ListFindings(ctx, &protoapi.ListFindingsRequest{})
	if err != nil || len(resp.GetFindings()) != 10 {
		t.Fatalf("deleting assessment did not restore finding: %v, %v", resp, err)
	}
}

func TestFakeTracksInventoryAndPropagatesErrors(t *testing.T) {
	c, inventory := testClient()
	inventory.response.Environments[0].Workloads = inventory.response.Environments[0].Workloads[:1]
	first, err := c.ListWorkloads(t.Context(), &protoapi.ListWorkloadsRequest{})
	if err != nil || len(first.GetWorkloads()) != 1 {
		t.Fatalf("unexpected initial inventory: %v, %v", first, err)
	}
	inventory.response.Environments[0].Workloads[0].Image = "repo/new@sha256:updated"
	second, err := c.ListWorkloads(t.Context(), &protoapi.ListWorkloadsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Workloads[0].ImageReference != "repo/new@sha256:updated" || second.Workloads[0].Image.Digest == first.Workloads[0].Image.Digest {
		t.Error("fake data did not follow updated workload inventory")
	}
	inventory.err = errors.New("inventory unavailable")
	if _, err := c.ListWorkloads(t.Context(), &protoapi.ListWorkloadsRequest{}); !errors.Is(err, inventory.err) {
		t.Errorf("inventory error was lost: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.ListFindings(ctx, &protoapi.ListFindingsRequest{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancellation was lost: %v", err)
	}
}

func TestFindingOrderAndInvalidCursor(t *testing.T) {
	c, _ := testClient()
	resp, err := c.ListFindings(t.Context(), &protoapi.ListFindingsRequest{OrderBy: &protoapi.FindingOrderBy{
		Field: protoapi.FindingOrderBy_FIELD_SEVERITY, Direction: protoapi.FindingOrderBy_DIRECTION_ASC,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var severities []protoapi.Severity
	for _, f := range resp.Findings {
		severities = append(severities, f.Severity)
	}
	want := []protoapi.Severity{
		protoapi.Severity_SEVERITY_UNKNOWN, protoapi.Severity_SEVERITY_UNKNOWN,
		protoapi.Severity_SEVERITY_LOW, protoapi.Severity_SEVERITY_LOW,
		protoapi.Severity_SEVERITY_MEDIUM, protoapi.Severity_SEVERITY_MEDIUM,
		protoapi.Severity_SEVERITY_HIGH, protoapi.Severity_SEVERITY_HIGH,
		protoapi.Severity_SEVERITY_CRITICAL, protoapi.Severity_SEVERITY_CRITICAL,
	}
	if !slices.Equal(want, severities) {
		t.Errorf("severity order (-want +got):\n%s", cmp.Diff(want, severities))
	}
	if _, err := c.ListFindings(t.Context(), &protoapi.ListFindingsRequest{PageToken: "invalid"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("invalid cursor error = %v", err)
	}
}
