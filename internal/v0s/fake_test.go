package v0s_test

import (
	"context"
	"testing"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v0s"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
)

type fakeInventory struct {
	inventoryapi.UnimplementedWorkloadsServer
}

func (fakeInventory) ListImages(context.Context, *inventoryapi.ListWorkloadImagesRequest) (*inventoryapi.ListWorkloadImagesResponse, error) {
	return &inventoryapi.ListWorkloadImagesResponse{
		Environments: []*inventoryapi.EnvironmentWorkloadImages{{
			Environment: "dev",
			Workloads: []*inventoryapi.WorkloadImage{
				{TeamSlug: "team-one", Name: "app", Image: "repo/app:1"},
				{TeamSlug: "team-one", Name: "no-errors", Image: "repo/clean:1"},
				{TeamSlug: "team-one", Name: "app-no-sbom", Image: "repo/no-sbom:1"},
			},
		}},
	}, nil
}

func TestFakeManagerServesVulnerabilityQueries(t *testing.T) {
	inventory := fakeInventory{}
	manager := v0s.NewFakeManager(inventory, false)
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx := authz.ContextWithActor(t.Context(), actor{}, nil)
	ctx = v0s.WithManager(ctx, manager)
	ctx = v0s.NewLoaderContext(ctx, func(ctx context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		return inventory.ListImages(ctx, &inventoryapi.ListWorkloadImagesRequest{})
	})
	first := 2
	findings, err := v0s.Findings(ctx, v0s.Scope{}, nil, &first, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings.Edges) != 2 || !findings.PageInfo.HasNextPage {
		t.Fatalf("unexpected findings page: %+v", findings)
	}
	if findings.Edges[0].Node.Epss == nil || findings.Edges[0].Node.Workload.Name != "app" {
		t.Errorf("fake finding did not convert correctly: %+v", findings.Edges[0].Node)
	}
	next, err := v0s.Findings(ctx, v0s.Scope{}, nil, &first, findings.PageInfo.EndCursor, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Edges) != 2 || next.Edges[0].Node.VulnerabilityIdentifier == findings.Edges[0].Node.VulnerabilityIdentifier {
		t.Error("fake finding cursor did not resume")
	}
	back, err := v0s.Findings(ctx, v0s.Scope{}, nil, nil, nil, &first, next.PageInfo.StartCursor, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Edges) != 2 || back.PageInfo.HasPreviousPage || !back.PageInfo.HasNextPage ||
		back.Edges[0].Cursor != findings.Edges[0].Cursor || back.Edges[1].Cursor != findings.Edges[1].Cursor {
		t.Fatalf("fake backward page did not return the original cursors: %+v", back)
	}
	summary, err := findings.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.FindingCount != 5 || summary.UniqueVulnerabilityCount != 5 ||
		summary.Coverage.TotalWorkloads != 3 || summary.Coverage.Scanned != 2 || summary.Coverage.NoSbom != 1 {
		t.Errorf("incorrect fake selection summary: %+v", summary)
	}
	if _, err := findings.Facets(ctx); err != nil {
		t.Fatalf("fake finding facets: %v", err)
	}
	vulnerabilities, err := findings.Vulnerabilities(ctx, &first, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(vulnerabilities.Edges) != 2 || vulnerabilities.PageInfo.TotalCount != 5 {
		t.Errorf("incorrect fake vulnerability overview: %+v", vulnerabilities)
	}
}
