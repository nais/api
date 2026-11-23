package v0s_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/v0s/protoapi"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc"
)

type actor struct{ authz.AuthenticatedUser }

type client struct {
	protoapi.VulnerabilityServiceClient
	protoapi.SuppressionServiceClient
	workloads  []*protoapi.Workload
	list       func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse
	countCalls int
}

func (c *client) ListWorkloads(context.Context, *protoapi.ListWorkloadsRequest, ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error) {
	return &protoapi.ListWorkloadsResponse{Workloads: c.workloads}, nil
}

func (c *client) ListFindings(_ context.Context, req *protoapi.ListFindingsRequest, _ ...grpc.CallOption) (*protoapi.ListFindingsResponse, error) {
	return c.list(req), nil
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
	first, err := v0s.Findings(ctx, nil, nil, new(2), nil)
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
	next, err := v0s.Findings(ctx, nil, nil, new(1), &after)
	if err != nil || len(next.Edges) != 1 || next.Edges[0].Node.VulnerabilityIdentifier != "CVE-2" {
		t.Fatalf("remaining finding was skipped: %+v, %v", next, err)
	}
	if _, err := v0s.Findings(ctx, nil, &v0s.VulnerabilityFindingFilter{OnlyWithFix: new(true)}, new(1), &after); err == nil {
		t.Fatal("cursor from another filter was accepted")
	}
	unbound := pagination.Cursor{Opaque: base64.RawURLEncoding.EncodeToString([]byte(`{"token":"first"}`))}
	if _, err := v0s.Findings(ctx, nil, nil, new(1), &unbound); err == nil {
		t.Fatal("cursor without selection or version was accepted")
	}
}

func TestWorkloadSummariesShareGroupedCounts(t *testing.T) {
	c := &client{}
	ctx := c.context(t, "app", "job")
	scope := &v0s.VulnerabilityScopeInput{
		Team: new(slug.Slug("team-one")), Environment: new("dev"), Workload: new("app"),
	}
	got, err := v0s.FindingSummary(ctx, scope, nil)
	if err != nil || got.TotalFindings != 3 {
		t.Fatalf("app summary: %+v, %v", got, err)
	}
	calls := c.countCalls
	scope.Workload = new("job")
	got, err = v0s.FindingSummary(ctx, scope, nil)
	if err != nil || got.TotalFindings != 1 {
		t.Fatalf("job summary: %+v, %v", got, err)
	}
	if c.countCalls != calls {
		t.Fatal("another workload summary repeated the same grouped backend requests")
	}
}
