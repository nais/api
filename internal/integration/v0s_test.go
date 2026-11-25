//go:build integration_test

package integration

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func paginationFindings() []*protoapi.Finding {
	findings := make([]*protoapi.Finding, 0, 5)
	for i, severity := range []protoapi.Severity{
		protoapi.Severity_SEVERITY_HIGH,
		protoapi.Severity_SEVERITY_LOW,
		protoapi.Severity_SEVERITY_HIGH,
		protoapi.Severity_SEVERITY_UNKNOWN,
	} {
		findings = append(findings, &protoapi.Finding{
			Workload:        &protoapi.WorkloadRef{Environment: "dev", Team: "team", Name: "app"},
			VulnerabilityId: []string{"CVE-B", "CVE-A", "CVE-C", "CVE-D"}[i],
			Severity:        severity,
			Purl:            "pkg:npm/example@1",
			PackageKey:      "pkg:npm/example",
		})
	}
	duplicate := proto.Clone(findings[0]).(*protoapi.Finding)
	duplicate.Workload.Name = "job"
	return append(findings, duplicate)
}

func TestV0sFindingCursorSurvivesDeletedBoundary(t *testing.T) {
	orders := []*protoapi.FindingOrderBy{nil}
	for _, field := range []protoapi.FindingOrderBy_Field{
		protoapi.FindingOrderBy_FIELD_SEVERITY,
		protoapi.FindingOrderBy_FIELD_VULNERABILITY_ID,
	} {
		for _, direction := range []protoapi.FindingOrderBy_Direction{
			protoapi.FindingOrderBy_DIRECTION_ASC, protoapi.FindingOrderBy_DIRECTION_DESC,
		} {
			orders = append(orders, &protoapi.FindingOrderBy{Field: field, Direction: direction})
		}
	}
	for _, order := range orders {
		t.Run(order.String(), func(t *testing.T) {
			c := &v0sClient{findings: paginationFindings()}
			req := &protoapi.ListFindingsRequest{Filter: &protoapi.FindingFilter{}, PageSize: 100, OrderBy: order}
			all, err := c.ListFindings(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			boundary := all.Findings[1]
			c.findings = slices.DeleteFunc(c.findings, func(f *protoapi.Finding) bool {
				return proto.Equal(f, boundary)
			})
			req.PageToken = all.Cursors[1]
			req.PageSize = 1
			for _, want := range all.Findings[2:] {
				page, err := c.ListFindings(t.Context(), req)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff([]*protoapi.Finding{want}, page.Findings, protocmp.Transform()); diff != "" {
					t.Fatalf("findings after deleted cursor boundary (-want +got):\n%s", diff)
				}
				req.PageToken = page.Cursors[0]
			}
			empty, err := c.ListFindings(t.Context(), req)
			if err != nil || len(empty.GetFindings()) != 0 {
				t.Fatalf("final continuation: %v, %v", empty, err)
			}
		})
	}
}

func TestV0sVulnerabilityCursorSurvivesDeletedBoundaryGroup(t *testing.T) {
	orders := []*protoapi.VulnerabilityOrderBy{nil}
	for _, field := range []protoapi.VulnerabilityOrderBy_Field{
		protoapi.VulnerabilityOrderBy_FIELD_SEVERITY,
		protoapi.VulnerabilityOrderBy_FIELD_VULNERABILITY_ID,
		protoapi.VulnerabilityOrderBy_FIELD_AFFECTED_WORKLOAD_COUNT,
	} {
		for _, direction := range []protoapi.VulnerabilityOrderBy_Direction{
			protoapi.VulnerabilityOrderBy_DIRECTION_ASC, protoapi.VulnerabilityOrderBy_DIRECTION_DESC,
		} {
			orders = append(orders, &protoapi.VulnerabilityOrderBy{Field: field, Direction: direction})
		}
	}
	for _, order := range orders {
		t.Run(order.String(), func(t *testing.T) {
			c := &v0sClient{findings: paginationFindings()}
			req := &protoapi.ListVulnerabilitiesRequest{Filter: &protoapi.FindingFilter{}, PageSize: 100, OrderBy: order}
			all, err := c.ListVulnerabilities(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			boundary := all.Vulnerabilities[1]
			c.findings = slices.DeleteFunc(c.findings, func(f *protoapi.Finding) bool {
				return f.VulnerabilityId == boundary.VulnerabilityId
			})
			req.PageToken = all.Cursors[1]
			req.PageSize = 1
			for _, want := range all.Vulnerabilities[2:] {
				page, err := c.ListVulnerabilities(t.Context(), req)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff([]*protoapi.VulnerabilitySummary{want}, page.Vulnerabilities, protocmp.Transform()); diff != "" {
					t.Fatalf("groups after deleted cursor boundary (-want +got):\n%s", diff)
				}
				if page.TotalCount != all.TotalCount-1 {
					t.Errorf("total after deleting group = %d, want %d", page.TotalCount, all.TotalCount-1)
				}
				req.PageToken = page.Cursors[0]
			}
			empty, err := c.ListVulnerabilities(t.Context(), req)
			if err != nil || len(empty.GetVulnerabilities()) != 0 {
				t.Fatalf("final continuation: %v, %v", empty, err)
			}
		})
	}
}
