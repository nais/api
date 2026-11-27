package v0s

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamscope"
	"github.com/nais/api/internal/workload"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
)

func TestAffectedWorkloadsAggregateBeforePagingAndBindSelection(t *testing.T) {
	items := []workload.Workload{
		orderedWorkload{name: "z-app", environment: "dev", kind: workload.TypeApplication},
		orderedWorkload{name: "a-clean", environment: "dev", kind: workload.TypeApplication},
		orderedWorkload{name: "b-missing", environment: "dev", kind: workload.TypeJob},
		orderedWorkload{name: "c-stale", environment: "dev", kind: workload.TypeApplication},
		orderedWorkload{name: "zz-job", environment: "dev", kind: workload.TypeJob},
	}
	current := map[workloadKey]*protoapi.Workload{}
	calls := 0
	removed := false
	c := &overviewClient{list: func(req *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
		calls++
		f := req.Overview.GetFindingFilter()
		if f.GetPackageType() != "npm" || f.GetSuppressed() != protoapi.SuppressedFilter_SUPPRESSED_FILTER_ONLY || !f.GetHasFix() {
			t.Errorf("lost finding/assessment filters: %v", f)
		}
		rows := []*protoapi.Workload{}
		for _, name := range []string{"zz-job", "z-app", "a-clean", "c-stale"} {
			row := &protoapi.Workload{
				Environment: "dev", Team: "team", Name: name, ImageReference: "repo/" + name + ":2",
				Image: &protoapi.Image{Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED},
				FindingCounts: &protoapi.WorkloadFindingCounts{
					FindingCount: 9, UniqueVulnerabilityCount: 1,
					SeverityCounts: &protoapi.VulnerabilitySeverityCounts{High: 1},
				},
			}
			if name == "a-clean" {
				row.FindingCounts = &protoapi.WorkloadFindingCounts{SeverityCounts: &protoapi.VulnerabilitySeverityCounts{}}
			}
			if removed && name == "z-app" {
				row.FindingCounts.FindingCount = 0
			}
			if name == "c-stale" {
				row.ImageReference = "old-image"
			} else {
				current[workloadKey{"dev", "team", name}] = row
			}
			rows = append(rows, row)
		}
		if req.PageToken == "" {
			return &protoapi.ListWorkloadsResponse{TotalCount: new(int64(4)), Workloads: rows[:1], NextPageToken: "rest"}, nil
		}
		if req.PageToken != "rest" {
			t.Fatalf("unexpected continuation token: %q", req.PageToken)
		}
		return &protoapi.ListWorkloadsResponse{TotalCount: new(int64(4)), Workloads: rows[1:]}, nil
	}}
	newContext := func() context.Context {
		ctx := authz.ContextWithActor(t.Context(), workloadReadActor{}, nil)
		ctx = WithManager(ctx, &Manager{Vulnerabilities: c})
		return NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
			t.Fatal("affected workloads must use their own image references, not untyped inventory")
			return nil, nil
		})
	}
	ctx := newContext()
	filter := &VulnerabilityFindingFilter{
		VulnerabilityIdentifiers: []string{"GHSA-test"}, PackageType: new("npm"),
		OnlyWithFix: new(true), Suppressed: new(SuppressedFilterOnly),
	}
	scope := Scope{Team: slug.Slug("team")}
	first, err := affectedWorkloads(ctx, scope, items, current, filter, "selection", new(1), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Edges) != 1 || first.Edges[0].Node.GetName() != "z-app" ||
		first.PageInfo.TotalCount != 2 || !first.PageInfo.HasNextPage {
		t.Fatalf("aggregation/pagination: %+v", first)
	}
	next, err := affectedWorkloads(ctx, scope, items, current, filter, "selection", new(2), first.PageInfo.EndCursor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Edges) != 1 || next.Edges[0].Node.GetName() != "zz-job" || next.PageInfo.HasNextPage || next.PageInfo.TotalCount != 2 {
		t.Fatalf("next page lost the later affected workload: %+v", next)
	}
	back, err := affectedWorkloads(ctx, scope, items, current, filter, "selection", nil, nil, new(20), next.PageInfo.StartCursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Edges) != 1 || back.Edges[0].Node.GetName() != "z-app" || back.PageInfo.HasPreviousPage ||
		!back.PageInfo.HasNextPage || back.PageInfo.TotalCount != 2 || back.Edges[0].Cursor != first.Edges[0].Cursor {
		t.Fatalf("backward page lost canonical order/total/cursor: %+v", back)
	}
	empty, err := affectedWorkloads(ctx, scope, items, current, filter, "selection", nil, nil, new(1), first.PageInfo.StartCursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Edges) != 0 || empty.PageInfo.HasPreviousPage || !empty.PageInfo.HasNextPage ||
		empty.PageInfo.StartCursor != nil || empty.PageInfo.EndCursor != nil || empty.PageInfo.TotalCount != 2 {
		t.Fatalf("empty before-first page: %+v", empty)
	}
	counts, err := next.Facets.LoadCoverage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(&workload.VulnerabilityCoverageCounts{TotalWorkloads: 5, Scanned: 3, Pending: 2}, counts); diff != "" {
		t.Fatalf("full-selection coverage (-want +got):\n%s", diff)
	}
	if calls != 2 {
		t.Fatalf("request-scoped aggregate cache bypassed: %d calls", calls)
	}
	for _, changed := range []string{"scope", "selection", "identifier", "assessment"} {
		t.Run(changed, func(t *testing.T) {
			s, f, binding := scope, *filter, "selection"
			switch changed {
			case "scope":
				s.Team = "other"
			case "selection":
				binding = "changed"
			case "identifier":
				f.VulnerabilityIdentifiers = []string{"CVE-other"}
			case "assessment":
				f.Suppressed = new(SuppressedFilterInclude)
			}
			if _, err := affectedWorkloads(ctx, s, items, current, &f, binding, nil, first.PageInfo.EndCursor, nil, nil); err == nil {
				t.Fatal("changed selection accepted a cursor")
			}
		})
	}
	removed = true
	resumed, err := affectedWorkloads(newContext(), scope, items, current, filter, "selection", nil, first.PageInfo.EndCursor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.Edges) != 1 || resumed.Edges[0].Node.GetName() != "zz-job" || resumed.PageInfo.TotalCount != 1 {
		t.Fatalf("cursor could not seek past a no-longer-affected boundary: %+v", resumed)
	}
}

func TestWorkloadSelectionBindingNormalizesMetadataAndTracksImages(t *testing.T) {
	items := []workload.Workload{
		orderedWorkload{name: "app", environment: "dev", kind: workload.TypeApplication},
		orderedWorkload{name: "job", environment: "dev", kind: workload.TypeJob},
	}
	filter := &VulnerabilityWorkloadFilter{
		TeamScope:    &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"b", "a", "b"}},
		Environments: []string{"prod", "dev"},
		Labels:       []*model.LabelFilter{{Key: "group", Value: new("")}, {Key: "present"}},
	}
	binding, err := workloadSelectionBinding(t.Context(), items, filter)
	if err != nil {
		t.Fatal(err)
	}
	equivalent := &VulnerabilityWorkloadFilter{
		TeamScope:    &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"a", "b"}},
		Environments: []string{"dev", "prod", "dev"},
		Labels:       []*model.LabelFilter{nil, {Key: ""}, {Key: "present"}, {Key: "group", Value: new("")}, {Key: "present"}},
	}
	got, err := workloadSelectionBinding(t.Context(), []workload.Workload{items[1], items[0], items[0]}, equivalent)
	if err != nil {
		t.Fatal(err)
	}
	if binding != got {
		t.Fatal("equivalent metadata/workload order changed the cursor binding")
	}
	for _, change := range []string{"image", "kind", "metadata", "teams"} {
		t.Run(change, func(t *testing.T) {
			w := items[0].(orderedWorkload)
			f := *filter
			switch change {
			case "image":
				w.image = "new-image"
			case "kind":
				w.kind = workload.TypeJob
			case "metadata":
				f.Labels = []*model.LabelFilter{{Key: "group"}, {Key: "present"}}
			case "teams":
				f.TeamScope = &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{}}
			}
			got, err := workloadSelectionBinding(t.Context(), []workload.Workload{w, items[1]}, &f)
			if err != nil {
				t.Fatal(err)
			}
			if got == binding {
				t.Fatal("selection change was not bound into cursors")
			}
		})
	}
}
