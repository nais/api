package v0s_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v0s"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc"
)

type actor struct{ authz.AuthenticatedUser }

type client struct {
	protoapi.VulnerabilityServiceClient
	protoapi.SuppressionServiceClient
	workloads       []*protoapi.Workload
	listWorkloads   func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error)
	list            func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse
	countCalls      int
	vulnerabilities func(*protoapi.ListVulnerabilitiesRequest) (*protoapi.ListVulnerabilitiesResponse, error)
	summary         func(*protoapi.GetSummaryRequest) (*protoapi.GetSummaryResponse, error)
}

func (c *client) ListWorkloads(_ context.Context, req *protoapi.ListWorkloadsRequest, _ ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error) {
	if c.listWorkloads != nil {
		return c.listWorkloads(req)
	}
	return &protoapi.ListWorkloadsResponse{Workloads: c.workloads}, nil
}

func (c *client) ListFindings(_ context.Context, req *protoapi.ListFindingsRequest, _ ...grpc.CallOption) (*protoapi.ListFindingsResponse, error) {
	return c.list(req), nil
}

func (c *client) ListVulnerabilities(_ context.Context, req *protoapi.ListVulnerabilitiesRequest, _ ...grpc.CallOption) (*protoapi.ListVulnerabilitiesResponse, error) {
	return c.vulnerabilities(req)
}

func (c *client) GetSummary(_ context.Context, req *protoapi.GetSummaryRequest, _ ...grpc.CallOption) (*protoapi.GetSummaryResponse, error) {
	return c.summary(req)
}

func (c *client) CountFindings(context.Context, *protoapi.CountFindingsRequest, ...grpc.CallOption) (*protoapi.CountFindingsResponse, error) {
	c.countCalls++
	return &protoapi.CountFindingsResponse{Buckets: []*protoapi.CountFindingsResponse_Bucket{
		{Key: "dev/team-one/app", Count: 3},
		{Key: "dev/team-one/job", Count: 1},
	}}, nil
}

func (c *client) context(t *testing.T, names ...string) context.Context {
	t.Helper()
	env := &inventoryapi.EnvironmentWorkloadImages{Environment: "dev"}
	for _, name := range names {
		image := "repo/" + name
		c.workloads = append(c.workloads, &protoapi.Workload{
			Environment: "dev", Team: "team-one", Name: name, ImageReference: image + ":1",
			Image: &protoapi.Image{Name: image, Digest: "sha256:" + name},
		})
		env.Workloads = append(env.Workloads, &inventoryapi.WorkloadImage{TeamSlug: "team-one", Name: name, Image: image + ":1"})
	}
	ctx := authz.ContextWithActor(t.Context(), actor{}, nil)
	ctx = v0s.WithManager(ctx, &v0s.Manager{Vulnerabilities: c, Suppressions: c})
	return v0s.NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		return &inventoryapi.ListWorkloadImagesResponse{Environments: []*inventoryapi.EnvironmentWorkloadImages{env}}, nil
	})
}

func TestFindingCursorResumesAnInteriorEdgeAfterEarlierFindingsDisappear(t *testing.T) {
	c := &client{}
	ctx := c.context(t, "app")
	ref := &protoapi.WorkloadRef{Environment: "dev", Team: "team-one", Name: "app"}
	finding := func(id string) *protoapi.Finding {
		return &protoapi.Finding{Workload: ref, Image: c.workloads[0].Image, VulnerabilityId: id, Purl: "pkg:npm/example@1"}
	}
	c.list = func(req *protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		if req.GetPageToken() != "" {
			t.Errorf("first request used cursor %q", req.GetPageToken())
		}
		return &protoapi.ListFindingsResponse{
			Findings: []*protoapi.Finding{finding("CVE-1"), finding("CVE-2"), finding("CVE-3")},
			Cursors:  []string{"first", "second", "third"},
		}
	}
	first, err := v0s.Findings(ctx, v0s.Scope{}, nil, new(2), nil, nil)
	if err != nil || len(first.Edges) != 2 || !first.PageInfo.HasNextPage {
		t.Fatalf("initial page: %+v, %v", first, err)
	}

	// The first finding was removed; resuming its cursor must still return the second.
	c.list = func(req *protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		if req.GetPageToken() != "first" {
			t.Errorf("resumed with %q, want the selected edge's token", req.GetPageToken())
		}
		return &protoapi.ListFindingsResponse{
			Findings: []*protoapi.Finding{finding("CVE-2"), finding("CVE-3")},
			Cursors:  []string{"second", "third"},
		}
	}
	after := first.Edges[0].Cursor
	next, err := v0s.Findings(ctx, v0s.Scope{}, nil, new(1), &after, nil)
	if err != nil || len(next.Edges) != 1 || next.Edges[0].Node.VulnerabilityIdentifier != "CVE-2" {
		t.Fatalf("remaining finding was skipped: %+v, %v", next, err)
	}
	if _, err := v0s.Findings(ctx, v0s.Scope{}, &v0s.VulnerabilityFindingFilter{OnlyWithFix: new(true)}, new(1), &after, nil); err == nil {
		t.Fatal("cursor from another filter was accepted")
	}
	unbound := pagination.Cursor{Keyset: base64.RawURLEncoding.EncodeToString([]byte(`{"token":"first"}`))}
	if _, err := v0s.Findings(ctx, v0s.Scope{}, nil, new(1), &unbound, nil); err == nil {
		t.Fatal("cursor without selection or version was accepted")
	}
}

func TestWorkloadFacetsShareGroupedCounts(t *testing.T) {
	c := &client{list: func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		return &protoapi.ListFindingsResponse{}
	}}
	ctx := c.context(t, "app", "job")
	scope := v0s.Scope{
		Team: slug.Slug("team-one"), Environment: "dev", Workload: "app",
	}

	page, err := v0s.Findings(ctx, scope, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	facets, err := page.Facets(ctx)
	if err != nil || facets.FixableFindings != 3 {
		t.Fatalf("app facets: %+v, %v", facets, err)
	}
	calls := c.countCalls
	scope.Workload = "job"
	page, err = v0s.Findings(ctx, scope, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	facets, err = page.Facets(ctx)
	if err != nil || facets.FixableFindings != 1 {
		t.Fatalf("job facets: %+v, %v", facets, err)
	}
	if c.countCalls != calls {
		t.Fatal("another workload's facets repeated the same grouped backend requests")
	}
}

func TestFindingIdentifierFilters(t *testing.T) {
	for _, tt := range []struct {
		name string
		ids  []string
		want []string
	}{
		{name: "single", ids: []string{"CVE-1"}, want: []string{"CVE-1"}},
		{name: "deduplicated single", ids: []string{"CVE-1", "CVE-1"}, want: []string{"CVE-1"}},
		{name: "multiple", ids: []string{"CVE-2", "CVE-1", "CVE-2"}, want: []string{"CVE-1", "CVE-2"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &client{list: func(req *protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
				if diff := cmp.Diff(tt.want, req.GetFilter().GetVulnerabilityIds()); diff != "" {
					t.Errorf("backend vulnerability IDs (-want +got):\n%s", diff)
				}
				return &protoapi.ListFindingsResponse{}
			}}
			ctx := c.context(t, "app")
			_, err := v0s.Findings(ctx, v0s.Scope{}, &v0s.VulnerabilityFindingFilter{
				VulnerabilityIdentifiers: tt.ids,
			}, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFindingOrderingAndPackageFacets(t *testing.T) {
	for _, field := range []v0s.VulnerabilityFindingOrderField{
		v0s.VulnerabilityFindingOrderFieldSeverity, v0s.VulnerabilityFindingOrderFieldIdentifier,
	} {
		for _, direction := range []model.OrderDirection{model.OrderDirectionAsc, model.OrderDirectionDesc} {
			t.Run(string(field)+"/"+string(direction), func(t *testing.T) {
				c := &client{}
				ctx := c.context(t, "app")
				c.workloads = append(c.workloads, &protoapi.Workload{
					Environment: "dev", Team: "team-one", Name: "removed", Image: c.workloads[0].Image,
				})
				c.list = func(req *protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
					wantField := protoapi.FindingOrderBy_FIELD_SEVERITY
					if field == v0s.VulnerabilityFindingOrderFieldIdentifier {
						wantField = protoapi.FindingOrderBy_FIELD_VULNERABILITY_ID
					}
					wantDirection := protoapi.FindingOrderBy_DIRECTION_ASC
					if direction == model.OrderDirectionDesc {
						wantDirection = protoapi.FindingOrderBy_DIRECTION_DESC
					}
					if req.GetOrderBy().GetField() != wantField || req.GetOrderBy().GetDirection() != wantDirection {
						t.Errorf("unexpected backend order: %v", req.GetOrderBy())
					}
					if req.GetFilter().GetPackageType() != "npm" {
						t.Errorf("package type = %q, want npm", req.GetFilter().GetPackageType())
					}
					selected := req.GetWorkloadSelector().GetWorkloads()
					if req.WorkloadSelector == nil || len(selected) != 1 || selected[0].GetName() != "app" {
						t.Errorf("selector must exclude removed workloads: %v", req.GetWorkloadSelector())
					}
					return &protoapi.ListFindingsResponse{
						Findings: []*protoapi.Finding{{
							Workload: selected[0], Image: c.workloads[0].Image, VulnerabilityId: "CVE-1",
						}},
						Cursors:           []string{"edge"},
						PackageTypeCounts: []*protoapi.PackageTypeCount{{Value: "npm", Count: 7}},
					}
				}
				filter := &v0s.VulnerabilityFindingFilter{PackageType: new("NPM")}
				order := &v0s.VulnerabilityFindingOrder{Field: field, Direction: direction}
				page, err := v0s.Findings(ctx, v0s.Scope{}, filter, new(1), nil, order)
				if err != nil {
					t.Fatal(err)
				}
				facets, err := page.Facets(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff([]model.StringFacetItem{{Value: "npm", Count: 7}}, facets.PackageTypes); diff != "" {
					t.Fatalf("full-selection package facets (-want +got):\n%s", diff)
				}
				after := page.Edges[0].Cursor
				filter.PackageType = new("npm")
				if _, err := v0s.Findings(ctx, v0s.Scope{}, filter, new(2), &after, order); err != nil {
					t.Fatalf("equivalent filter and changed page size rejected: %v", err)
				}
				changed := *order
				if direction == model.OrderDirectionAsc {
					changed.Direction = model.OrderDirectionDesc
				} else {
					changed.Direction = model.OrderDirectionAsc
				}
				if _, err := v0s.Findings(ctx, v0s.Scope{}, filter, nil, &after, &changed); err == nil {
					t.Error("cursor accepted for different direction")
				}
				if field == v0s.VulnerabilityFindingOrderFieldSeverity {
					changed.Field = v0s.VulnerabilityFindingOrderFieldIdentifier
				} else {
					changed.Field = v0s.VulnerabilityFindingOrderFieldSeverity
				}
				changed.Direction = direction
				if _, err := v0s.Findings(ctx, v0s.Scope{}, filter, nil, &after, &changed); err == nil {
					t.Error("cursor accepted for different order field")
				}
				if _, err := v0s.Findings(ctx, v0s.Scope{}, filter, nil, &after, nil); err == nil {
					t.Error("sorted cursor accepted for default order")
				}
				filter.PackageType = new("maven")
				if _, err := v0s.Findings(ctx, v0s.Scope{}, filter, nil, &after, order); err == nil {
					t.Error("cursor accepted for different package type")
				}
			})
		}
	}
}

func TestFindingEmptyWorkloadSelection(t *testing.T) {
	c := &client{list: func(req *protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		if req.WorkloadSelector == nil || len(req.WorkloadSelector.Workloads) != 0 {
			t.Errorf("empty inventory must send explicit empty selector: %v", req.WorkloadSelector)
		}
		return &protoapi.ListFindingsResponse{}
	}}
	page, err := v0s.Findings(c.context(t), v0s.Scope{}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	facets, err := page.Facets(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(facets.PackageTypes) != 0 {
		t.Errorf("empty selection has package facets: %v", facets.PackageTypes)
	}
}
