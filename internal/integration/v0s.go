//go:build integration_test

package integration

import (
	"cmp"
	"context"
	"encoding/base64"
	"slices"
	"strings"
	"sync"

	"github.com/nais/api/internal/v0s"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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
	c.addWorkloadOverviewFixtures()
	slices.SortFunc(c.findings, func(a, b *protoapi.Finding) int {
		return strings.Compare(v0sFindingKey(a), v0sFindingKey(b))
	})
	return &v0s.Manager{Vulnerabilities: c, Suppressions: c, SuppressionsEnabled: true}
}

func (c *v0sClient) addWorkloadOverviewFixtures() {
	for _, environment := range []string{"dev", "staging"} {
		names := []string{"alpha"}
		if environment == "dev" {
			names = append(names, "zeta", "clean", "failed", "no-sbom", "unresolved", "stale", "other-group", "missing-key", "nonempty", "not-empty", "collision")
		}
		for _, name := range names {
			imageName := "repo/" + environment + "/" + name
			if name == "collision" {
				imageName += "-job"
			}
			w := &protoapi.Workload{
				Environment: environment, Team: "v0s-overview", Name: name, ImageReference: imageName + ":2",
				Image: &protoapi.Image{Name: imageName, Digest: "sha256:" + environment + "-" + name, Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED},
			}
			switch name {
			case "failed":
				w.Image.Status = protoapi.ImageStatus_IMAGE_STATUS_FAILED
			case "no-sbom":
				w.Image.Status = protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM
			case "unresolved":
				w.Image, w.ResolveError = nil, "cannot resolve"
			case "stale":
				w.ImageReference = imageName + ":1"
			}
			c.workloads = append(c.workloads, w)
			add := func(id, ecosystem, pkg string, severity protoapi.Severity) {
				c.findings = append(c.findings, &protoapi.Finding{
					Workload: &protoapi.WorkloadRef{Environment: environment, Team: w.Team, Name: name}, Image: w.Image,
					VulnerabilityId: id, PackageKey: "pkg:" + ecosystem + "/" + pkg, Purl: "pkg:" + ecosystem + "/" + pkg + "@1",
					InstalledVersion: "1", FixedVersion: "2", Severity: severity, KnownExploited: true,
				})
			}
			if name == "alpha" && environment == "dev" {
				add("CVE-shared", "npm", "first", protoapi.Severity_SEVERITY_HIGH)
				add("CVE-shared", "npm", "second", protoapi.Severity_SEVERITY_CRITICAL)
				add("CVE-go", "golang", "example", protoapi.Severity_SEVERITY_CRITICAL)
			} else if name == "zeta" {
				add("CVE-shared", "npm", "first", protoapi.Severity_SEVERITY_CRITICAL)
			} else if name != "clean" && name != "failed" && name != "no-sbom" && name != "unresolved" {
				add("CVE-outside", "npm", "outside", protoapi.Severity_SEVERITY_CRITICAL)
			}
		}
	}
}

func (c *v0sClient) ListWorkloads(_ context.Context, req *protoapi.ListWorkloadsRequest, _ ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error) {
	resp := &protoapi.ListWorkloadsResponse{}
	for _, w := range c.workloads {
		ref := &protoapi.WorkloadRef{Environment: w.Environment, Team: w.Team, Name: w.Name}
		if (req.Team != "" && req.Team != w.Team) || (req.Environment != "" && req.Environment != w.Environment) ||
			(req.Workload != "" && req.Workload != w.Name) || !v0sWorkloadSelected(ref, req.WorkloadSelector) {
			continue
		}
		value := proto.Clone(w).(*protoapi.Workload)
		if req.Overview != nil {
			filter := req.Overview.GetFindingFilter()
			groups := c.vulnerabilityGroups(filter, &protoapi.WorkloadSelector{Workloads: []*protoapi.WorkloadRef{ref}})
			counts := &protoapi.WorkloadFindingCounts{
				UniqueVulnerabilityCount: int64(len(groups)), SeverityCounts: &protoapi.VulnerabilitySeverityCounts{},
			}
			for _, group := range groups {
				counts.FindingCount += group.FindingCount
				switch group.Severity {
				case protoapi.Severity_SEVERITY_CRITICAL:
					counts.SeverityCounts.Critical++
				case protoapi.Severity_SEVERITY_HIGH:
					counts.SeverityCounts.High++
				case protoapi.Severity_SEVERITY_MEDIUM:
					counts.SeverityCounts.Medium++
				case protoapi.Severity_SEVERITY_LOW:
					counts.SeverityCounts.Low++
				default:
					counts.SeverityCounts.Unknown++
				}
				if slices.ContainsFunc(c.findings, func(f *protoapi.Finding) bool {
					return f.VulnerabilityId == group.VulnerabilityId && f.KnownExploited &&
						v0sMatches(f, filter) && v0sWorkloadSelected(f.Workload, &protoapi.WorkloadSelector{Workloads: []*protoapi.WorkloadRef{ref}})
				}) {
					counts.UniqueKevVulnerabilityCount++
				}
			}
			value.FindingCounts = counts
		}
		resp.Workloads = append(resp.Workloads, value)
	}
	if req.Overview != nil {
		resp.TotalCount = new(int64(len(resp.Workloads)))
	}
	return resp, nil
}

func v0sFindingKey(f *protoapi.Finding) string {
	w := f.GetWorkload()
	return strings.Join([]string{w.Environment, w.Team, w.Name, f.VulnerabilityId, f.Purl}, "\x00")
}

func v0sMatches(f *protoapi.Finding, filter *protoapi.FindingFilter) bool {
	packageType, _, _ := strings.Cut(strings.TrimPrefix(f.PackageKey, "pkg:"), "/")
	return (filter.Team == "" || filter.Team == f.Workload.Team) &&
		(filter.Environment == "" || filter.Environment == f.Workload.Environment) &&
		(filter.Workload == "" || filter.Workload == f.Workload.Name) &&
		(len(filter.VulnerabilityIds) == 0 || slices.Contains(filter.VulnerabilityIds, f.VulnerabilityId)) &&
		(filter.Package == "" || filter.Package == f.Purl || filter.Package == f.PackageKey) &&
		(filter.PackageType == "" || strings.EqualFold(filter.PackageType, packageType)) &&
		(len(filter.Severities) == 0 || slices.Contains(filter.Severities, f.Severity)) &&
		(!filter.HasFix || f.FixedVersion != "") && (!filter.OnlyKnownExploited || f.KnownExploited) &&
		filter.Suppressed != protoapi.SuppressedFilter_SUPPRESSED_FILTER_ONLY
}

func v0sWorkloadSelected(w *protoapi.WorkloadRef, selector *protoapi.WorkloadSelector) bool {
	return selector == nil || slices.ContainsFunc(selector.Workloads, func(selected *protoapi.WorkloadRef) bool {
		return w.Environment == selected.Environment && w.Team == selected.Team && w.Name == selected.Name
	})
}

func v0sCompareFindings(a, b *protoapi.Finding, order *protoapi.FindingOrderBy) int {
	if order != nil {
		var compared int
		switch order.Field {
		case protoapi.FindingOrderBy_FIELD_SEVERITY:
			rank := func(f *protoapi.Finding) int {
				if f.Severity <= protoapi.Severity_SEVERITY_UNKNOWN && order.Direction == protoapi.FindingOrderBy_DIRECTION_ASC {
					return int(protoapi.Severity_SEVERITY_CRITICAL) + 1
				}
				return int(f.Severity)
			}
			compared = cmp.Compare(rank(a), rank(b))
		case protoapi.FindingOrderBy_FIELD_VULNERABILITY_ID:
			compared = strings.Compare(a.VulnerabilityId, b.VulnerabilityId)
		}
		if order.Direction == protoapi.FindingOrderBy_DIRECTION_DESC {
			compared = -compared
		}
		if compared != 0 {
			return compared
		}
		if compared = strings.Compare(a.VulnerabilityId, b.VulnerabilityId); compared != 0 {
			return compared
		}
	}
	return strings.Compare(v0sFindingKey(a), v0sFindingKey(b))
}

func v0sEncodeCursor(boundary proto.Message) (string, error) {
	data, err := proto.Marshal(boundary)
	if err != nil {
		return "", status.Errorf(codes.Internal, "encode fake v0s cursor: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func v0sDecodeCursor(token string, boundary proto.Message) error {
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "decode fake v0s cursor: %v", err)
	}
	if err := proto.Unmarshal(data, boundary); err != nil {
		return status.Errorf(codes.InvalidArgument, "decode fake v0s cursor boundary: %v", err)
	}
	return nil
}

func (c *v0sClient) ListFindings(_ context.Context, req *protoapi.ListFindingsRequest, _ ...grpc.CallOption) (*protoapi.ListFindingsResponse, error) {
	var boundary *protoapi.Finding
	if req.PageToken != "" {
		boundary = &protoapi.Finding{}
		if err := v0sDecodeCursor(req.PageToken, boundary); err != nil {
			return nil, err
		}
	}
	resp := &protoapi.ListFindingsResponse{}
	matching := make([]*protoapi.Finding, 0)
	packageCounts := map[string]int64{}
	for _, f := range c.findings {
		if v0sMatches(f, req.Filter) && v0sWorkloadSelected(f.Workload, req.WorkloadSelector) {
			matching = append(matching, f)
			packageType, _, _ := strings.Cut(strings.TrimPrefix(f.PackageKey, "pkg:"), "/")
			packageCounts[packageType]++
		}
	}
	for value, count := range packageCounts {
		resp.PackageTypeCounts = append(resp.PackageTypeCounts, &protoapi.PackageTypeCount{Value: value, Count: count})
	}
	slices.SortFunc(matching, func(a, b *protoapi.Finding) int { return v0sCompareFindings(a, b, req.OrderBy) })
	for _, f := range matching {
		if boundary != nil && v0sCompareFindings(f, boundary, req.OrderBy) <= 0 {
			continue
		}
		if len(resp.Findings) == int(req.PageSize) {
			resp.NextPageToken = resp.Cursors[len(resp.Cursors)-1]
			break
		}
		cursor, err := v0sEncodeCursor(&protoapi.Finding{
			Workload: f.Workload, VulnerabilityId: f.VulnerabilityId, Purl: f.Purl, Severity: f.Severity,
		})
		if err != nil {
			return nil, err
		}
		resp.Findings = append(resp.Findings, f)
		resp.Cursors = append(resp.Cursors, cursor)
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

func (c *v0sClient) vulnerabilityGroups(filter *protoapi.FindingFilter, selector *protoapi.WorkloadSelector) []*protoapi.VulnerabilitySummary {
	groups := map[string]*protoapi.VulnerabilitySummary{}
	workloads := map[string]map[string]bool{}
	for _, f := range c.findings {
		if !v0sMatches(f, filter) || !v0sWorkloadSelected(f.Workload, selector) {
			continue
		}
		group := groups[f.VulnerabilityId]
		if group == nil {
			group = &protoapi.VulnerabilitySummary{VulnerabilityId: f.VulnerabilityId, Severity: protoapi.Severity_SEVERITY_UNKNOWN}
			groups[f.VulnerabilityId] = group
			workloads[f.VulnerabilityId] = map[string]bool{}
		}
		group.FindingCount++
		group.Severity = max(group.Severity, f.Severity)
		w := f.Workload
		workloads[f.VulnerabilityId][strings.Join([]string{w.Environment, w.Team, w.Name}, "\x00")] = true
	}
	result := make([]*protoapi.VulnerabilitySummary, 0, len(groups))
	for id, group := range groups {
		group.AffectedWorkloadCount = int64(len(workloads[id]))
		result = append(result, group)
	}
	return result
}

func v0sCompareVulnerabilityGroups(a, b *protoapi.VulnerabilitySummary, order *protoapi.VulnerabilityOrderBy) int {
	if order == nil {
		order = &protoapi.VulnerabilityOrderBy{Field: protoapi.VulnerabilityOrderBy_FIELD_SEVERITY, Direction: protoapi.VulnerabilityOrderBy_DIRECTION_DESC}
	}
	var compared int
	switch order.Field {
	case protoapi.VulnerabilityOrderBy_FIELD_VULNERABILITY_ID:
		compared = strings.Compare(a.VulnerabilityId, b.VulnerabilityId)
	case protoapi.VulnerabilityOrderBy_FIELD_AFFECTED_WORKLOAD_COUNT:
		compared = cmp.Compare(a.AffectedWorkloadCount, b.AffectedWorkloadCount)
	default:
		if a.Severity == protoapi.Severity_SEVERITY_UNKNOWN && b.Severity != a.Severity {
			return 1
		}
		if b.Severity == protoapi.Severity_SEVERITY_UNKNOWN && a.Severity != b.Severity {
			return -1
		}
		compared = cmp.Compare(a.Severity, b.Severity)
	}
	if order.Direction == protoapi.VulnerabilityOrderBy_DIRECTION_DESC {
		compared = -compared
	}
	if compared != 0 {
		return compared
	}
	return strings.Compare(a.VulnerabilityId, b.VulnerabilityId)
}

func (c *v0sClient) ListVulnerabilities(_ context.Context, req *protoapi.ListVulnerabilitiesRequest, _ ...grpc.CallOption) (*protoapi.ListVulnerabilitiesResponse, error) {
	var boundary *protoapi.VulnerabilitySummary
	if req.PageToken != "" {
		boundary = &protoapi.VulnerabilitySummary{}
		if err := v0sDecodeCursor(req.PageToken, boundary); err != nil {
			return nil, err
		}
	}
	groups := c.vulnerabilityGroups(req.Filter, req.WorkloadSelector)
	slices.SortFunc(groups, func(a, b *protoapi.VulnerabilitySummary) int {
		return v0sCompareVulnerabilityGroups(a, b, req.OrderBy)
	})
	resp := &protoapi.ListVulnerabilitiesResponse{TotalCount: int64(len(groups))}
	for _, group := range groups {
		if boundary != nil && v0sCompareVulnerabilityGroups(group, boundary, req.OrderBy) <= 0 {
			continue
		}
		if len(resp.Vulnerabilities) == int(req.PageSize) {
			resp.NextPageToken = resp.Cursors[len(resp.Cursors)-1]
			break
		}
		cursor, err := v0sEncodeCursor(group)
		if err != nil {
			return nil, err
		}
		resp.Vulnerabilities = append(resp.Vulnerabilities, group)
		resp.Cursors = append(resp.Cursors, cursor)
	}
	return resp, nil
}

func (c *v0sClient) GetSummary(_ context.Context, req *protoapi.GetSummaryRequest, _ ...grpc.CallOption) (*protoapi.GetSummaryResponse, error) {
	filter := proto.Clone(req.FindingFilter).(*protoapi.FindingFilter)
	filter.Team, filter.Environment, filter.Workload = req.Team, req.Environment, req.Workload
	groups := c.vulnerabilityGroups(filter, req.WorkloadSelector)
	resp := &protoapi.GetSummaryResponse{
		UniqueVulnerabilityCount: int64(len(groups)), SeverityCounts: &protoapi.VulnerabilitySeverityCounts{},
		Coverage: &protoapi.WorkloadCoverage{},
	}
	for _, group := range groups {
		resp.FindingCount += group.FindingCount
		switch group.Severity {
		case protoapi.Severity_SEVERITY_CRITICAL:
			resp.SeverityCounts.Critical++
		case protoapi.Severity_SEVERITY_HIGH:
			resp.SeverityCounts.High++
		case protoapi.Severity_SEVERITY_MEDIUM:
			resp.SeverityCounts.Medium++
		case protoapi.Severity_SEVERITY_LOW:
			resp.SeverityCounts.Low++
		default:
			resp.SeverityCounts.Unknown++
		}
	}
	affected, kevWorkloads, kevIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, f := range c.findings {
		if !v0sMatches(f, filter) || !v0sWorkloadSelected(f.Workload, req.WorkloadSelector) {
			continue
		}
		w := f.Workload
		key := strings.Join([]string{w.Environment, w.Team, w.Name}, "\x00")
		affected[key] = true
		if f.KnownExploited {
			kevWorkloads[key], kevIDs[f.VulnerabilityId] = true, true
		}
	}
	resp.AffectedWorkloadCount, resp.KevAffectedWorkloadCount = int64(len(affected)), int64(len(kevWorkloads))
	resp.UniqueKevVulnerabilityCount = int64(len(kevIDs))
	for _, w := range c.workloads {
		if (req.Team != "" && req.Team != w.Team) || (req.Environment != "" && req.Environment != w.Environment) ||
			(req.Workload != "" && req.Workload != w.Name) ||
			!v0sWorkloadSelected(&protoapi.WorkloadRef{Environment: w.Environment, Team: w.Team, Name: w.Name}, req.WorkloadSelector) {
			continue
		}
		resp.Coverage.TotalWorkloadCount++
		switch {
		case w.Image == nil:
			resp.Coverage.UnresolvedImageCount++
		case w.Image.Status == protoapi.ImageStatus_IMAGE_STATUS_SCANNED:
			resp.Coverage.ScannedCount++
		case w.Image.Status == protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM:
			resp.Coverage.NoSbomCount++
		case w.Image.Status == protoapi.ImageStatus_IMAGE_STATUS_FAILED:
			resp.Coverage.FailedCount++
		default:
			resp.Coverage.PendingCount++
		}
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
		Id:         req.Id,
		References: []string{"https://example.com/advisories/" + req.Id},
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
