package v0s_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v0s"
	protoapi "github.com/nais/v0s/api/v0s/v1"
)

func TestVulnerabilityOverviewUsesFullSelectionAndOwnCursors(t *testing.T) {
	c := &client{list: func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
		return &protoapi.ListFindingsResponse{}
	}}
	ctx := c.context(t, "app", "job")
	c.vulnerabilities = func(req *protoapi.ListVulnerabilitiesRequest) (*protoapi.ListVulnerabilitiesResponse, error) {
		if req.Filter.GetPackageType() != "npm" || req.Filter.GetSuppressed() != protoapi.SuppressedFilter_SUPPRESSED_FILTER_EXCLUDE {
			t.Errorf("finding filter was not inherited: %v", req.Filter)
		}
		if req.WorkloadSelector == nil || len(req.WorkloadSelector.Workloads) != 2 {
			t.Errorf("full workload selection was not inherited: %v", req.WorkloadSelector)
		}
		if req.OrderBy.GetField() != protoapi.VulnerabilityOrderBy_FIELD_SEVERITY ||
			req.OrderBy.GetDirection() != protoapi.VulnerabilityOrderBy_DIRECTION_DESC {
			t.Errorf("unexpected default group order: %v", req.OrderBy)
		}
		a := &protoapi.VulnerabilitySummary{VulnerabilityId: "CVE-A", Severity: protoapi.Severity_SEVERITY_HIGH, FindingCount: 9, AffectedWorkloadCount: 2}
		b := &protoapi.VulnerabilitySummary{VulnerabilityId: "CVE-B", Severity: protoapi.Severity_SEVERITY_LOW, FindingCount: 4, AffectedWorkloadCount: 1}
		switch req.PageToken {
		case "":
			return &protoapi.ListVulnerabilitiesResponse{
				Vulnerabilities: []*protoapi.VulnerabilitySummary{a, b}, Cursors: []string{"a", "b"}, TotalCount: 2,
			}, nil
		case "a":
			return &protoapi.ListVulnerabilitiesResponse{Vulnerabilities: []*protoapi.VulnerabilitySummary{b}, Cursors: []string{"b"}, TotalCount: 2}, nil
		case "b":
			return &protoapi.ListVulnerabilitiesResponse{TotalCount: 2}, nil
		default:
			t.Fatalf("unexpected backend cursor %q", req.PageToken)
			return nil, nil
		}
	}
	filter := &v0s.VulnerabilityFindingFilter{PackageType: new("NPM")}
	page, err := v0s.Findings(ctx, v0s.Scope{}, filter, new(1), nil, nil, nil, &v0s.VulnerabilityFindingOrder{
		Field: v0s.VulnerabilityFindingOrderFieldIdentifier, Direction: model.OrderDirectionAsc,
	})
	if err != nil {
		t.Fatal(err)
	}
	groups, err := page.Vulnerabilities(ctx, new(1), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if groups.PageInfo.TotalCount != 2 || !groups.PageInfo.HasNextPage || len(groups.Edges) != 1 {
		t.Fatalf("full unique total/page: %+v", groups)
	}
	value := groups.Edges[0].Node
	if value.FindingCount != 9 || value.AffectedWorkloadCount != 2 || value.Severity != v0s.VulnerabilitySeverityHigh {
		t.Fatalf("server aggregate was not preserved: %+v", value)
	}
	parent, err := v0s.Findings(ctx, v0s.Scope{}, filter, new(2), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	explicitDefault := &v0s.VulnerabilityOverviewOrder{
		Field: v0s.VulnerabilityOverviewOrderFieldSeverity, Direction: model.OrderDirectionDesc,
	}
	next, err := parent.Vulnerabilities(ctx, new(2), groups.PageInfo.EndCursor, nil, nil, explicitDefault)
	if err != nil || len(next.Edges) != 1 || next.Edges[0].Node.VulnerabilityIdentifier != "CVE-B" {
		t.Fatalf("group cursor depended on parent finding order/page: %+v, %v", next, err)
	}
	empty, err := parent.Vulnerabilities(ctx, new(1), next.PageInfo.EndCursor, nil, nil, nil)
	if err != nil || len(empty.Edges) != 0 || empty.PageInfo.TotalCount != 2 {
		t.Fatalf("empty page lost full unique total: %+v, %v", empty, err)
	}
	for _, changed := range []*v0s.VulnerabilityOverviewOrder{
		{Field: v0s.VulnerabilityOverviewOrderFieldSeverity, Direction: model.OrderDirectionAsc},
		{Field: v0s.VulnerabilityOverviewOrderFieldIdentifier, Direction: model.OrderDirectionDesc},
	} {
		if _, err := parent.Vulnerabilities(ctx, nil, groups.PageInfo.EndCursor, nil, nil, changed); err == nil {
			t.Error("group cursor accepted with changed ordering")
		}
	}
	scoped, err := v0s.Findings(ctx, v0s.Scope{Team: slug.Slug("team-one"), Environment: "dev", Workload: "job"}, filter, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scoped.Vulnerabilities(ctx, nil, groups.PageInfo.EndCursor, nil, nil, nil); err == nil {
		t.Error("group cursor accepted in a different workload scope")
	}
	filtered, err := v0s.Findings(ctx, v0s.Scope{}, &v0s.VulnerabilityFindingFilter{PackageType: new("maven")}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := filtered.Vulnerabilities(ctx, nil, groups.PageInfo.EndCursor, nil, nil, nil); err == nil {
		t.Error("group cursor accepted with a different finding filter")
	}
}

func TestVulnerabilityOverviewForwardsAllOrderingModes(t *testing.T) {
	for _, field := range []struct {
		input v0s.VulnerabilityOverviewOrderField
		want  protoapi.VulnerabilityOrderBy_Field
	}{
		{v0s.VulnerabilityOverviewOrderFieldSeverity, protoapi.VulnerabilityOrderBy_FIELD_SEVERITY},
		{v0s.VulnerabilityOverviewOrderFieldIdentifier, protoapi.VulnerabilityOrderBy_FIELD_VULNERABILITY_ID},
		{v0s.VulnerabilityOverviewOrderFieldAffectedWorkloadCount, protoapi.VulnerabilityOrderBy_FIELD_AFFECTED_WORKLOAD_COUNT},
	} {
		for _, direction := range []model.OrderDirection{model.OrderDirectionAsc, model.OrderDirectionDesc} {
			t.Run(string(field.input)+"/"+string(direction), func(t *testing.T) {
				c := &client{
					list: func(*protoapi.ListFindingsRequest) *protoapi.ListFindingsResponse {
						return &protoapi.ListFindingsResponse{}
					},
					vulnerabilities: func(req *protoapi.ListVulnerabilitiesRequest) (*protoapi.ListVulnerabilitiesResponse, error) {
						wantDirection := protoapi.VulnerabilityOrderBy_DIRECTION_ASC
						if direction == model.OrderDirectionDesc {
							wantDirection = protoapi.VulnerabilityOrderBy_DIRECTION_DESC
						}
						if req.OrderBy.GetField() != field.want || req.OrderBy.GetDirection() != wantDirection {
							t.Errorf("unexpected backend order: %v", req.OrderBy)
						}
						return &protoapi.ListVulnerabilitiesResponse{}, nil
					},
				}
				ctx := c.context(t, "app")
				page, err := v0s.Findings(ctx, v0s.Scope{}, nil, nil, nil, nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				_, err = page.Vulnerabilities(ctx, nil, nil, nil, nil, &v0s.VulnerabilityOverviewOrder{Field: field.input, Direction: direction})
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
