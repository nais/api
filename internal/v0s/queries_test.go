package v0s_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/v0s/protoapi"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type actor struct{ authz.AuthenticatedUser }

func (actor) Identity() string       { return "actor-1" }
func (actor) IsServiceAccount() bool { return true }
func (actor) IsGitHubActions()       {}

type client struct {
	protoapi.VulnerabilityServiceClient
	protoapi.SuppressionServiceClient
	workloads                             []*protoapi.Workload
	findings                              []*protoapi.Finding
	assessments                           []*protoapi.Suppression
	vulnerability                         *protoapi.Vulnerability
	err                                   error
	mu                                    sync.Mutex
	workloadCalls, countCalls, writeCalls int
}

func (c *client) ListWorkloads(_ context.Context, r *protoapi.ListWorkloadsRequest, _ ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workloadCalls++
	result := &protoapi.ListWorkloadsResponse{}
	for _, w := range c.workloads {
		if (r.Team == "" || r.Team == w.Team) && (r.Environment == "" || r.Environment == w.Environment) {
			result.Workloads = append(result.Workloads, w)
		}
	}
	return result, c.err
}

func matches(f *protoapi.Finding, filter *protoapi.FindingFilter) bool {
	return (filter.Team == "" || f.Workload.Team == filter.Team) &&
		(filter.Environment == "" || f.Workload.Environment == filter.Environment) &&
		(filter.Workload == "" || f.Workload.Name == filter.Workload) &&
		(filter.VulnerabilityId == "" || f.VulnerabilityId == filter.VulnerabilityId) &&
		(filter.Package == "" || f.Purl == filter.Package || f.PackageKey == filter.Package) &&
		(len(filter.Severities) == 0 || slices.Contains(filter.Severities, f.Severity)) &&
		(!filter.HasFix || f.FixedVersion != "") && (!filter.OnlyKnownExploited || f.KnownExploited) &&
		(filter.Suppressed == protoapi.SuppressedFilter_SUPPRESSED_FILTER_INCLUDE ||
			(filter.Suppressed == protoapi.SuppressedFilter_SUPPRESSED_FILTER_ONLY && f.Suppression != nil) ||
			(filter.Suppressed == protoapi.SuppressedFilter_SUPPRESSED_FILTER_EXCLUDE && f.Suppression == nil))
}

func (c *client) ListFindings(_ context.Context, r *protoapi.ListFindingsRequest, _ ...grpc.CallOption) (*protoapi.ListFindingsResponse, error) {
	values := make([]*protoapi.Finding, 0)
	for _, f := range c.findings {
		if matches(f, r.Filter) && findingKey(f) > r.PageToken {
			values = append(values, f)
		}
	}
	slices.SortFunc(values, func(a, b *protoapi.Finding) int { return strings.Compare(findingKey(a), findingKey(b)) })
	end := min(int(r.PageSize), len(values))
	next := ""
	if end < len(values) {
		next = findingKey(values[end-1])
	}
	cursors := make([]string, end)
	for i, value := range values[:end] {
		cursors[i] = findingKey(value)
	}
	return &protoapi.ListFindingsResponse{Findings: values[:end], Cursors: cursors, NextPageToken: next}, c.err
}

func findingKey(f *protoapi.Finding) string {
	w := f.GetWorkload()
	return strings.Join([]string{w.GetEnvironment(), w.GetTeam(), w.GetName(), f.GetVulnerabilityId(), f.GetPurl()}, "\x00")
}

func (c *client) CountFindings(_ context.Context, r *protoapi.CountFindingsRequest, _ ...grpc.CallOption) (*protoapi.CountFindingsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.countCalls++
	if r.GroupBy != protoapi.CountFindingsRequest_GROUP_BY_WORKLOAD {
		return nil, fmt.Errorf("expected workload grouping")
	}
	counts := map[string]int64{}
	for _, f := range c.findings {
		if matches(f, r.Filter) {
			w := f.Workload
			counts[w.Environment+"/"+w.Team+"/"+w.Name]++
		}
	}
	result := &protoapi.CountFindingsResponse{}
	for key, count := range counts {
		result.Buckets = append(result.Buckets, &protoapi.CountFindingsResponse_Bucket{Key: key, Count: count})
	}
	return result, c.err
}

func (c *client) GetVulnerability(context.Context, *protoapi.GetVulnerabilityRequest, ...grpc.CallOption) (*protoapi.GetVulnerabilityResponse, error) {
	return &protoapi.GetVulnerabilityResponse{Vulnerability: c.vulnerability}, c.err
}

func (c *client) ListSuppressions(_ context.Context, r *protoapi.ListSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.ListSuppressionsResponse, error) {
	values := make([]*protoapi.Suppression, 0)
	for _, s := range c.assessments {
		if (r.Team == "" || r.Team == s.Team) && (r.Environment == "" || r.Environment == s.Environment) &&
			(r.Workload == "" || r.Workload == s.Workload) && (r.ImageName == "" || r.ImageName == s.ImageName) &&
			(r.VulnerabilityId == "" || r.VulnerabilityId == s.VulnerabilityId) && suppressionKey(s) > r.PageToken {
			values = append(values, s)
		}
	}
	slices.SortFunc(values, func(a, b *protoapi.Suppression) int { return strings.Compare(suppressionKey(a), suppressionKey(b)) })
	end := min(int(r.PageSize), len(values))
	next := ""
	if end < len(values) {
		next = suppressionKey(values[end-1])
	}
	cursors := make([]string, end)
	for i, value := range values[:end] {
		cursors[i] = suppressionKey(value)
	}
	return &protoapi.ListSuppressionsResponse{Suppressions: values[:end], Cursors: cursors, NextPageToken: next}, c.err
}

func suppressionKey(s *protoapi.Suppression) string {
	return strings.Join([]string{s.Team, s.Environment, s.Workload, s.ImageName, s.PackageKey, s.VulnerabilityId}, "\x00")
}

func (c *client) SetSuppressions(context.Context, *protoapi.SetSuppressionsRequest, ...grpc.CallOption) (*protoapi.SetSuppressionsResponse, error) {
	c.writeCalls++
	return &protoapi.SetSuppressionsResponse{}, c.err
}

func (c *client) DeleteSuppressions(context.Context, *protoapi.DeleteSuppressionsRequest, ...grpc.CallOption) (*protoapi.DeleteSuppressionsResponse, error) {
	c.writeCalls++
	return &protoapi.DeleteSuppressionsResponse{}, c.err
}

func contextFor(t *testing.T, c *client, inventory []*inventoryapi.WorkloadImage) context.Context {
	t.Helper()
	ctx := authz.ContextWithActor(t.Context(), actor{}, nil)
	ctx = v0s.WithManager(ctx, &v0s.Manager{Vulnerabilities: c, Suppressions: c})
	return v0s.NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		return &inventoryapi.ListWorkloadImagesResponse{Environments: []*inventoryapi.EnvironmentWorkloadImages{{
			Environment: "dev", Workloads: inventory,
		}}}, nil
	})
}

func fixture() (*client, []*inventoryapi.WorkloadImage) {
	c := &client{}
	inventory := []*inventoryapi.WorkloadImage{}
	for _, name := range []string{"scanned", "fresh", "stale", "no-sbom", "failed", "unresolved", "rescan"} {
		inventory = append(inventory, &inventoryapi.WorkloadImage{
			TeamSlug: "team-one", Name: name, Image: "repo/" + name + ":2",
		})
		if name == "fresh" {
			continue
		}
		w := &protoapi.Workload{
			Environment: "dev", Team: "team-one", Name: name, ImageReference: "repo/" + name + ":2",
			Image: &protoapi.Image{Name: "repo/" + name, Digest: "sha256:" + name, Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED},
		}
		switch name {
		case "stale":
			w.ImageReference = "repo/stale:1"
		case "no-sbom":
			w.Image.Status = protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM
		case "failed":
			w.Image.Status = protoapi.ImageStatus_IMAGE_STATUS_FAILED
			w.Image.Error = "scan failed"
		case "unresolved":
			w.Image = nil
			w.ResolveError = "could not resolve"
		case "rescan":
			w.Image.Error = "rescan failed"
			w.Image.ScannedAt = timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
		}
		c.workloads = append(c.workloads, w)
	}
	c.workloads = append(c.workloads, &protoapi.Workload{
		Environment: "dev", Team: "team-one", Name: "removed", ImageReference: "repo/removed:1",
		Image: &protoapi.Image{Name: "repo/removed", Digest: "sha256:removed", Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED},
	})
	for i, item := range []struct {
		name                 string
		severity             protoapi.Severity
		fix, kev, suppressed bool
	}{
		{"scanned", protoapi.Severity_SEVERITY_HIGH, false, true, false},
		{"scanned", protoapi.Severity_SEVERITY_CRITICAL, true, false, false},
		{"scanned", protoapi.Severity_SEVERITY_LOW, true, true, true},
		{"scanned", protoapi.Severity_SEVERITY_UNKNOWN, false, false, false},
		{"rescan", protoapi.Severity_SEVERITY_MEDIUM, true, true, false},
		{"stale", protoapi.Severity_SEVERITY_CRITICAL, true, true, false},
		{"removed", protoapi.Severity_SEVERITY_CRITICAL, true, true, false},
	} {
		f := &protoapi.Finding{
			Workload:        &protoapi.WorkloadRef{Environment: "dev", Team: "team-one", Name: item.name},
			Image:           &protoapi.Image{Name: "repo/" + item.name, Digest: "sha256:" + item.name},
			VulnerabilityId: fmt.Sprintf("CVE-%d", i), Purl: "pkg:npm/example@1.0", PackageKey: "pkg:npm/example",
			Severity: item.severity, KnownExploited: item.kev,
		}
		if item.fix {
			f.FixedVersion = "2.0"
		}
		if item.suppressed {
			f.Suppression = &protoapi.Suppression{
				Team: "team-one", Environment: "dev", Workload: item.name,
				State: protoapi.SuppressionState_SUPPRESSION_STATE_ACCEPTED_RISK, UpdatedAt: timestamppb.Now(),
			}
		}
		c.findings = append(c.findings, f)
	}
	return c, inventory
}

func TestCoverageUsesCurrentInventory(t *testing.T) {
	c, inventory := fixture()
	ctx := contextFor(t, c, inventory)
	got, err := v0s.Coverage(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := &v0s.VulnerabilityCoverageCounts{TotalWorkloads: 7, Scanned: 2, Pending: 2, NoSbom: 1, Failed: 1, UnresolvedImage: 1}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("coverage (-want +got):\n%s", diff)
	}
	for _, name := range []string{"fresh", "stale", "rescan"} {
		w := &application.Application{Base: workload.Base{Name: name, EnvironmentName: "dev", TeamSlug: "team-one"}}
		value, err := v0s.WorkloadCoverage(ctx, w)
		if err != nil {
			t.Fatal(err)
		}
		if name == "rescan" {
			if value.Status != v0s.VulnerabilityCoverageStatusScanned || value.Error == nil || value.ScannedAt == nil {
				t.Fatalf("failed rescan lost successful coverage: %+v", value)
			}
		} else if value.Status != v0s.VulnerabilityCoverageStatusPending || value.ImageDigest != nil {
			t.Fatalf("new or obsolete image must be pending: %+v", value)
		}
	}
	if c.workloadCalls != 2 {
		t.Fatalf("coverage was not shared across workload fields: %d calls", c.workloadCalls)
	}
}

func TestFindingsResumeAnyEdgeAndExcludeObsoleteImages(t *testing.T) {
	c, inventory := fixture()
	ctx := contextFor(t, c, inventory)
	page, err := v0s.Findings(ctx, nil, nil, new(2), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Edges) != 2 || !page.PageInfo.HasNextPage || page.Edges[1].Node.FixedVersion != nil ||
		page.PageInfo.PageStart() != 1 || page.PageInfo.PageEnd() != 2 {
		t.Fatalf("unexpected first page: %+v", page)
	}
	next, err := v0s.Findings(ctx, nil, nil, new(1), &page.Edges[0].Cursor)
	if err != nil || len(next.Edges) != 1 || next.Edges[0].Node.VulnerabilityIdentifier != "CVE-0" ||
		!next.PageInfo.HasPreviousPage || next.PageInfo.PageStart() != 2 || next.PageInfo.PageEnd() != 2 {
		t.Fatalf("resuming an interior edge: %+v, %v", next, err)
	}
	var identifiers []string
	for {
		for _, edge := range page.Edges {
			identifiers = append(identifiers, edge.Node.VulnerabilityIdentifier)
		}
		if !page.PageInfo.HasNextPage {
			break
		}
		page, err = v0s.Findings(ctx, nil, nil, new(1), page.PageInfo.EndCursor)
		if err != nil {
			t.Fatal(err)
		}
	}
	if diff := cmp.Diff([]string{"CVE-4", "CVE-0", "CVE-1", "CVE-3"}, identifiers); diff != "" {
		t.Fatalf("current findings (-want +got):\n%s", diff)
	}
	_, err = v0s.Findings(ctx, nil, &v0s.VulnerabilityFindingFilter{OnlyWithFix: new(true)}, new(1), next.PageInfo.EndCursor)
	if err == nil {
		t.Fatal("cursor from another filter was accepted")
	}
	unbound := pagination.Cursor{Opaque: base64.RawURLEncoding.EncodeToString([]byte(`{"token":"unbound"}`))}
	if _, err := v0s.Findings(ctx, nil, nil, new(1), &unbound); err == nil {
		t.Fatal("a cursor without selection or version was accepted")
	}
}

func TestFindingsResumeAfterEarlierFindingDisappears(t *testing.T) {
	c, inventory := fixture()
	c.findings = c.findings[:2]
	first, err := v0s.Findings(contextFor(t, c, inventory), nil, nil, new(1), nil)
	if err != nil || len(first.Edges) != 1 {
		t.Fatalf("initial page: %+v, %v", first, err)
	}
	c.findings = c.findings[1:]
	next, err := v0s.Findings(contextFor(t, c, inventory), nil, nil, new(1), first.PageInfo.EndCursor)
	if err != nil || len(next.Edges) != 1 || next.Edges[0].Node.VulnerabilityIdentifier != "CVE-1" || next.PageInfo.HasNextPage {
		t.Fatalf("remaining finding was skipped: %+v, %v", next, err)
	}
}

func TestInvalidFindingPageDoesNotReadInventory(t *testing.T) {
	for _, size := range []int{0, 201} {
		c, inventory := fixture()
		if _, err := v0s.Findings(contextFor(t, c, inventory), nil, nil, &size, nil); err == nil {
			t.Fatalf("invalid page size %d accepted", size)
		}
		if c.workloadCalls != 0 {
			t.Fatal("invalid pagination read the backend inventory")
		}
	}
}

func TestVulnerabilityNodesCanBeRefetched(t *testing.T) {
	c, inventory := fixture()
	c.findings = c.findings[:2]
	c.vulnerability = &protoapi.Vulnerability{Id: "CVE-1", Title: "advisory"}
	c.assessments = []*protoapi.Suppression{{
		Team: "team-one", Environment: "dev", Workload: "removed", ImageName: "repo/old",
		PackageKey: "pkg:npm/example", VulnerabilityId: "CVE-1",
		State: protoapi.SuppressionState_SUPPRESSION_STATE_ACCEPTED_RISK, UpdatedAt: timestamppb.Now(),
	}}
	ctx := contextFor(t, c, inventory)
	findings, err := v0s.Findings(ctx, nil, nil, new(1), nil)
	if err != nil {
		t.Fatal(err)
	}
	advisory, err := v0s.VulnerabilityByIdentifier(ctx, "CVE-1")
	if err != nil {
		t.Fatal(err)
	}
	assessments, err := v0s.Suppressions(ctx, nil, nil, new(1), nil)
	if err != nil {
		t.Fatal(err)
	}
	type identifiedNode interface {
		model.Node
		ID() ident.Ident
	}
	for _, node := range []identifiedNode{findings.Nodes()[0], advisory, assessments.Nodes()[0]} {
		got, err := ident.GetByIdent(ctx, ident.FromString(node.ID().String()))
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(node, got); diff != "" {
			t.Fatalf("node lookup lost the resource (-want +got):\n%s", diff)
		}
	}
	for _, w := range c.workloads {
		if w.GetName() == "scanned" {
			w.Image.Digest = "sha256:new"
		}
	}
	if _, err := ident.GetByIdent(contextFor(t, c, inventory), findings.Nodes()[0].ID()); err == nil {
		t.Fatal("an obsolete-image finding could still be fetched by ID")
	}
}

func TestSummaryCountsFullFilteredCurrentSelection(t *testing.T) {
	c, inventory := fixture()
	ctx := contextFor(t, c, inventory)
	got, err := v0s.FindingSummary(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := &v0s.VulnerabilityFindingSummary{
		TotalFindings: 4, FixableFindings: 2, KnownExploitedFindings: 2,
		CountsBySeverity: &v0s.VulnerabilitySeverityCounts{Unknown: 1, Medium: 1, High: 1, Critical: 1},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("summary (-want +got):\n%s", diff)
	}
	got, err = v0s.FindingSummary(ctx, nil, &v0s.VulnerabilityFindingFilter{
		Severities:  []v0s.VulnerabilitySeverity{v0s.VulnerabilitySeverityHigh, v0s.VulnerabilitySeverityCritical},
		OnlyWithFix: new(true),
	})
	if err != nil || got.TotalFindings != 1 || got.CountsBySeverity.High != 0 ||
		got.CountsBySeverity.Critical != 1 || got.FixableFindings != 1 || got.KnownExploitedFindings != 0 {
		t.Fatalf("summary did not preserve filters: %+v, %v", got, err)
	}
	got, err = v0s.FindingSummary(ctx, nil, &v0s.VulnerabilityFindingFilter{Suppressed: new(v0s.SuppressedFilterInclude)})
	if err != nil || got.TotalFindings != 5 || got.FixableFindings != 3 || got.KnownExploitedFindings != 3 {
		t.Fatalf("including assessments: %+v, %v", got, err)
	}
}

func TestWorkloadSummariesShareGroupedCounts(t *testing.T) {
	c, inventory := fixture()
	ctx := contextFor(t, c, inventory)
	var countCalls, workloadCalls int
	for i, name := range []string{"scanned", "rescan"} {
		got, err := v0s.FindingSummary(ctx, &v0s.VulnerabilityScopeInput{
			Team: new(slug.Slug("team-one")), Environment: new("dev"), Workload: &name,
		}, nil)
		want := int64(3)
		if name == "rescan" {
			want = 1
		}
		if err != nil || got.TotalFindings != want {
			t.Fatalf("workload summary: %+v, %v", got, err)
		}
		if i == 0 {
			countCalls, workloadCalls = c.countCalls, c.workloadCalls
		} else if c.countCalls != countCalls || c.workloadCalls != workloadCalls {
			t.Fatal("another workload summary repeated the same grouped backend requests")
		}
	}
}

func TestAdvisoryPreservesOptionalValuesAndBackendErrors(t *testing.T) {
	c := &client{vulnerability: &protoapi.Vulnerability{Id: "CVE-1", Cvss: []*protoapi.Cvss{
		{Source: "source", Version: "3.x"}, {Source: "source", Version: "3.1", Score: new(0.0), Vector: new("vector")},
	}, Epss: &protoapi.Epss{Score: 0, Date: "2026-01-02"}}}
	got, err := v0s.VulnerabilityByIdentifier(contextFor(t, c, nil), "alias")
	if err != nil || got.Cvss[0].Score != nil || got.Cvss[0].Vector != nil ||
		got.Cvss[1].Score == nil || *got.Cvss[1].Score != 0 || got.Epss.Date.String() != "2026-01-02" {
		t.Fatalf("optional advisory values: %+v, %v", got, err)
	}
	c.err = status.Error(codes.NotFound, "unknown")
	if got, err := v0s.VulnerabilityByIdentifier(contextFor(t, c, nil), "missing"); got != nil || err != nil {
		t.Fatalf("unknown identifier: %+v, %v", got, err)
	}
	c.err = status.Error(codes.Unavailable, "catalog unavailable")
	if got, err := v0s.VulnerabilityByIdentifier(contextFor(t, c, nil), "CVE-1"); got != nil || err == nil {
		t.Fatal("backend outage was presented as a missing advisory")
	}
	if got, err := v0s.Coverage(contextFor(t, c, nil), nil); got != nil || err == nil {
		t.Fatal("backend outage was presented as zero coverage")
	}
}

func TestScopeAndReadAuthorization(t *testing.T) {
	c, inventory := fixture()
	ctx := contextFor(t, c, inventory)
	for _, scope := range []*v0s.VulnerabilityScopeInput{
		{Environment: new("")},
		{Workload: new("scanned")},
		{Team: new(slug.Slug("team-one")), Workload: new("scanned")},
	} {
		if _, err := v0s.Coverage(ctx, scope); err == nil {
			t.Fatalf("invalid scope accepted: %+v", scope)
		}
	}
	if c.workloadCalls != 0 {
		t.Fatal("invalid scope reached v0s")
	}
	if _, err := v0s.Coverage(context.Background(), nil); err == nil {
		t.Fatal("unauthenticated read accepted")
	}
	ctx = v0s.WithManager(authz.ContextWithActor(t.Context(), actor{}, nil), nil)
	ctx = v0s.NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		t.Fatal("disabled v0s attempted an inventory read")
		return nil, nil
	})
	if _, err := v0s.Coverage(ctx, nil); err == nil {
		t.Fatal("unconfigured backend presented zero coverage")
	}
}

func TestSuppressionsRemainReadableWithoutActiveFindingsAndWritesStayGated(t *testing.T) {
	c := &client{assessments: []*protoapi.Suppression{{
		Team: "team-one", Environment: "dev", Workload: "removed", ImageName: "repo/old",
		PackageKey: "pkg:npm/example", VulnerabilityId: "CVE-1",
		State: protoapi.SuppressionState_SUPPRESSION_STATE_ACCEPTED_RISK, UpdatedAt: timestamppb.Now(),
	}}}
	ctx := contextFor(t, c, nil)
	got, err := v0s.Suppressions(ctx, nil, nil, new(1), nil)
	if err != nil || len(got.Edges) != 1 || got.Edges[0].Node.Workload != "removed" {
		t.Fatalf("assessment without active finding: %+v, %v", got, err)
	}
	key := &v0s.VulnerabilitySuppressionKeyInput{
		Environment: "dev", Workload: "removed", ImageName: "repo/old", PackageKey: "pkg:npm/example", VulnerabilityIdentifier: "CVE-1",
	}
	for _, enabled := range []bool{false, true} {
		ctx := v0s.WithManager(authz.ContextWithActor(t.Context(), actor{}, nil), &v0s.Manager{
			Vulnerabilities: c, Suppressions: c, SuppressionsEnabled: enabled,
		})
		if _, err := v0s.SetSuppressions(ctx, v0s.SetVulnerabilitySuppressionsInput{Team: "team-one", Items: []*v0s.NewVulnerabilitySuppressionInput{{
			Key: key, State: v0s.VulnerabilitySuppressionStateAcceptedRisk, Reason: "reason",
		}}}); err == nil {
			t.Fatal("disabled or unauthorized set was accepted")
		}
		if _, err := v0s.DeleteSuppressions(ctx, v0s.DeleteVulnerabilitySuppressionsInput{Team: "team-one", Keys: []*v0s.VulnerabilitySuppressionKeyInput{key}}); err == nil {
			t.Fatal("disabled or unauthorized delete was accepted")
		}
	}
	if c.writeCalls != 0 {
		t.Fatal("gated or unauthorized batch reached v0s")
	}
}
