//go:build integration_test

package integration

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/nais/api/internal/v0s"
	protoapi "github.com/nais/v0s-api/v0s/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type v0sClient struct {
	protoapi.VulnerabilityServiceClient
	protoapi.SuppressionServiceClient
	workloads    []*protoapi.Workload
	findings     []*protoapi.Finding
	mu           sync.Mutex
	suppressions []*protoapi.Suppression
}

func newV0sManager(config *Config) *v0s.Manager {
	if !config.V0s {
		return nil
	}
	c := &v0sClient{}
	for _, name := range []string{"app", "stale", "removed"} {
		ref := "repo/" + name + ":2"
		if name == "stale" {
			ref = "repo/stale:1"
		}
		image := &protoapi.Image{Name: "repo/" + name, Digest: "sha256:" + name, Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED}
		c.workloads = append(c.workloads, &protoapi.Workload{
			Environment: "dev", Team: "v0s-team", Name: name, ImageReference: ref, Image: image,
		})
		c.findings = append(c.findings, &protoapi.Finding{
			Workload: &protoapi.WorkloadRef{Environment: "dev", Team: "v0s-team", Name: name}, Image: image,
			VulnerabilityId: "CVE-1", Purl: "pkg:npm/example@1", PackageKey: "pkg:npm/example",
			InstalledVersion: "1", Severity: protoapi.Severity_SEVERITY_HIGH, KnownExploited: true,
		})
	}
	c.findings = append(c.findings, &protoapi.Finding{
		Workload: c.findings[0].Workload, Image: c.findings[0].Image,
		VulnerabilityId: "CVE-2", Purl: "pkg:npm/other@1", PackageKey: "pkg:npm/other",
		InstalledVersion: "1", FixedVersion: "2", Severity: protoapi.Severity_SEVERITY_CRITICAL,
	})
	slices.SortFunc(c.findings, func(a, b *protoapi.Finding) int {
		return strings.Compare(v0sFindingKey(a), v0sFindingKey(b))
	})
	return &v0s.Manager{Vulnerabilities: c, Suppressions: c, SuppressionsEnabled: true}
}

func (c *v0sClient) ListWorkloads(_ context.Context, req *protoapi.ListWorkloadsRequest, _ ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error) {
	resp := &protoapi.ListWorkloadsResponse{}
	for _, w := range c.workloads {
		if (req.Team == "" || req.Team == w.Team) && (req.Environment == "" || req.Environment == w.Environment) {
			resp.Workloads = append(resp.Workloads, w)
		}
	}
	return resp, nil
}

func v0sFindingKey(f *protoapi.Finding) string {
	w := f.GetWorkload()
	return strings.Join([]string{w.Environment, w.Team, w.Name, f.VulnerabilityId, f.Purl}, "\x00")
}

func v0sMatches(f *protoapi.Finding, filter *protoapi.FindingFilter) bool {
	return (filter.Team == "" || filter.Team == f.Workload.Team) &&
		(filter.Environment == "" || filter.Environment == f.Workload.Environment) &&
		(filter.Workload == "" || filter.Workload == f.Workload.Name) &&
		((filter.VulnerabilityId == "" && len(filter.VulnerabilityIds) == 0) ||
			filter.VulnerabilityId == f.VulnerabilityId || slices.Contains(filter.VulnerabilityIds, f.VulnerabilityId)) &&
		(filter.Package == "" || filter.Package == f.Purl || filter.Package == f.PackageKey) &&
		(len(filter.Severities) == 0 || slices.Contains(filter.Severities, f.Severity)) &&
		(!filter.HasFix || f.FixedVersion != "") && (!filter.OnlyKnownExploited || f.KnownExploited) &&
		filter.Suppressed != protoapi.SuppressedFilter_SUPPRESSED_FILTER_ONLY
}

func (c *v0sClient) ListFindings(_ context.Context, req *protoapi.ListFindingsRequest, _ ...grpc.CallOption) (*protoapi.ListFindingsResponse, error) {
	resp := &protoapi.ListFindingsResponse{}
	for _, f := range c.findings {
		key := v0sFindingKey(f)
		if key <= req.PageToken || !v0sMatches(f, req.Filter) {
			continue
		}
		if len(resp.Findings) == int(req.PageSize) {
			resp.NextPageToken = resp.Cursors[len(resp.Cursors)-1]
			break
		}
		resp.Findings = append(resp.Findings, f)
		resp.Cursors = append(resp.Cursors, key)
	}
	return resp, nil
}

func (c *v0sClient) CountFindings(_ context.Context, req *protoapi.CountFindingsRequest, _ ...grpc.CallOption) (*protoapi.CountFindingsResponse, error) {
	counts := map[string]int64{}
	for _, f := range c.findings {
		if v0sMatches(f, req.Filter) {
			w := f.Workload
			counts[w.Environment+"/"+w.Team+"/"+w.Name]++
		}
	}
	resp := &protoapi.CountFindingsResponse{}
	for key, count := range counts {
		resp.Buckets = append(resp.Buckets, &protoapi.CountFindingsResponse_Bucket{Key: key, Count: count})
	}
	return resp, nil
}

func (*v0sClient) GetVulnerability(_ context.Context, req *protoapi.GetVulnerabilityRequest, _ ...grpc.CallOption) (*protoapi.GetVulnerabilityResponse, error) {
	switch req.Id {
	case "ambiguous":
		return nil, status.Error(codes.InvalidArgument, "alias names several vulnerabilities")
	case "missing":
		return nil, status.Error(codes.NotFound, "unknown vulnerability")
	}
	return &protoapi.GetVulnerabilityResponse{Vulnerability: &protoapi.Vulnerability{
		Id: req.Id, Cvss: []*protoapi.Cvss{{Source: "source", Version: "3.x"}, {Source: "source", Version: "3.1", Score: new(0.0)}},
	}}, nil
}

func (c *v0sClient) ListSuppressions(_ context.Context, req *protoapi.ListSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.ListSuppressionsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp := &protoapi.ListSuppressionsResponse{}
	for _, s := range c.suppressions {
		key := v0sSuppressionKey(s)
		if key <= req.PageToken || (req.Team != "" && req.Team != s.Team) ||
			(req.Environment != "" && req.Environment != s.Environment) || (req.Workload != "" && req.Workload != s.Workload) ||
			(req.ImageName != "" && req.ImageName != s.ImageName) || (req.VulnerabilityId != "" && req.VulnerabilityId != s.VulnerabilityId) {
			continue
		}
		resp.Suppressions = append(resp.Suppressions, s)
		resp.Cursors = append(resp.Cursors, key)
	}
	return resp, nil
}

func v0sSuppressionKey(s *protoapi.Suppression) string {
	return strings.Join([]string{s.Team, s.Environment, s.Workload, s.ImageName, s.PackageKey, s.VulnerabilityId}, "\x00")
}

func (c *v0sClient) SetSuppressions(_ context.Context, req *protoapi.SetSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.SetSuppressionsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp := &protoapi.SetSuppressionsResponse{Suppressions: make([]*protoapi.Suppression, 0, len(req.Suppressions))}
	for _, item := range req.Suppressions {
		key := item.Key
		s := &protoapi.Suppression{
			Team: req.Team, Environment: key.Environment, Workload: key.Workload, ImageName: key.ImageName,
			PackageKey: key.Package, VulnerabilityId: key.VulnerabilityId, State: item.State, Detail: item.Detail,
			UpdatedBy: req.Actor, UpdatedAt: timestamppb.Now(),
		}
		c.suppressions = append(c.suppressions, s)
		resp.Suppressions = append(resp.Suppressions, s)
	}
	return resp, nil
}

func (c *v0sClient) DeleteSuppressions(_ context.Context, req *protoapi.DeleteSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.DeleteSuppressionsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp := &protoapi.DeleteSuppressionsResponse{}
	c.suppressions = slices.DeleteFunc(c.suppressions, func(s *protoapi.Suppression) bool {
		for _, key := range req.Keys {
			if req.Team == s.Team && key.Environment == s.Environment && key.Workload == s.Workload &&
				key.ImageName == s.ImageName && key.Package == s.PackageKey && key.VulnerabilityId == s.VulnerabilityId {
				resp.Deleted = append(resp.Deleted, s)
				return true
			}
		}
		return false
	})
	return resp, nil
}
