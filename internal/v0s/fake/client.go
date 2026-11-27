package fake

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	inventory    inventoryapi.WorkloadsServer
	scannedAt    time.Time
	mu           sync.RWMutex
	suppressions map[suppressionKey]*protoapi.Suppression
}

var (
	_ protoapi.VulnerabilityServiceClient = (*Client)(nil)
	_ protoapi.SuppressionServiceClient   = (*Client)(nil)
)

func NewClient(inventory inventoryapi.WorkloadsServer) *Client {
	return &Client{
		inventory: inventory, scannedAt: time.Now(),
		suppressions: make(map[suppressionKey]*protoapi.Suppression),
	}
}

func (c *Client) ListWorkloads(ctx context.Context, req *protoapi.ListWorkloadsRequest, _ ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error) {
	workloads, findings, err := c.data(ctx)
	if err != nil {
		return nil, err
	}
	workloads = slices.DeleteFunc(workloads, func(w *protoapi.Workload) bool {
		return !matchesScope(w.Environment, w.Team, w.Name, req.Environment, req.Team, req.Workload) ||
			!selected(req.WorkloadSelector, w.Environment, w.Team, w.Name)
	})
	for _, w := range workloads {
		if req.Overview != nil {
			selector := &protoapi.WorkloadSelector{Workloads: []*protoapi.WorkloadRef{{
				Environment: w.Environment, Team: w.Team, Name: w.Name,
			}}}
			summary := summarize(filterFindings(findings, req.Overview.FindingFilter, selector))
			w.FindingCounts = &protoapi.WorkloadFindingCounts{
				FindingCount: summary.FindingCount, UniqueVulnerabilityCount: summary.UniqueVulnerabilityCount,
				UniqueKevVulnerabilityCount: summary.UniqueKevVulnerabilityCount, SeverityCounts: summary.SeverityCounts,
			}
		}
	}
	total := int64(len(workloads))
	rows, _, next, _, err := paginate(workloads, req.PageSize, req.PageToken, protoapi.PageDirection_PAGE_DIRECTION_FORWARD)
	if err != nil {
		return nil, err
	}
	resp := &protoapi.ListWorkloadsResponse{Workloads: rows, NextPageToken: next}
	if req.Overview != nil {
		resp.TotalCount = &total
	}
	return resp, nil
}

func (c *Client) ListFindings(ctx context.Context, req *protoapi.ListFindingsRequest, _ ...grpc.CallOption) (*protoapi.ListFindingsResponse, error) {
	_, findings, err := c.data(ctx)
	if err != nil {
		return nil, err
	}
	findings = filterFindings(findings, req.Filter, req.WorkloadSelector)
	slices.SortFunc(findings, func(a, b *protoapi.Finding) int {
		if req.OrderBy.GetField() == protoapi.FindingOrderBy_FIELD_UNSPECIFIED {
			return cmp.Or(strings.Compare(workloadKey(a.Workload), workloadKey(b.Workload)),
				strings.Compare(a.VulnerabilityId, b.VulnerabilityId), strings.Compare(a.Purl, b.Purl))
		}
		var order int
		if req.OrderBy.GetField() == protoapi.FindingOrderBy_FIELD_VULNERABILITY_ID {
			order = strings.Compare(a.VulnerabilityId, b.VulnerabilityId)
			if req.OrderBy.GetDirection() == protoapi.FindingOrderBy_DIRECTION_DESC {
				order = -order
			}
		} else {
			order = compareSeverity(a.Severity, b.Severity, req.OrderBy.GetDirection() == protoapi.FindingOrderBy_DIRECTION_ASC)
		}
		return cmp.Or(order, strings.Compare(a.VulnerabilityId, b.VulnerabilityId),
			strings.Compare(workloadKey(a.Workload), workloadKey(b.Workload)), strings.Compare(a.Purl, b.Purl))
	})
	counts := make(map[string]int64)
	for _, f := range findings {
		counts[packageType(f.PackageKey)]++
	}
	facets := make([]*protoapi.PackageTypeCount, 0, len(counts))
	for value, count := range counts {
		facets = append(facets, &protoapi.PackageTypeCount{Value: value, Count: count})
	}
	slices.SortFunc(facets, func(a, b *protoapi.PackageTypeCount) int { return strings.Compare(a.Value, b.Value) })
	rows, cursors, next, info, err := paginate(findings, req.PageSize, req.PageToken, req.PageDirection)
	return &protoapi.ListFindingsResponse{Findings: rows, Cursors: cursors, NextPageToken: next, PackageTypeCounts: facets, PageInfo: info}, err
}

func (c *Client) ListVulnerabilities(ctx context.Context, req *protoapi.ListVulnerabilitiesRequest, _ ...grpc.CallOption) (*protoapi.ListVulnerabilitiesResponse, error) {
	_, findings, err := c.data(ctx)
	if err != nil {
		return nil, err
	}
	findings = filterFindings(findings, req.Filter, req.WorkloadSelector)
	groups := make(map[string]*protoapi.VulnerabilitySummary)
	workloads := make(map[string]map[string]bool)
	for _, f := range findings {
		if groups[f.VulnerabilityId] == nil {
			groups[f.VulnerabilityId] = &protoapi.VulnerabilitySummary{VulnerabilityId: f.VulnerabilityId}
			workloads[f.VulnerabilityId] = make(map[string]bool)
		}
		g := groups[f.VulnerabilityId]
		g.Severity = max(g.Severity, f.Severity)
		g.FindingCount++
		workloads[f.VulnerabilityId][workloadKey(f.Workload)] = true
	}
	rows := make([]*protoapi.VulnerabilitySummary, 0, len(groups))
	for id, g := range groups {
		g.AffectedWorkloadCount = int64(len(workloads[id]))
		rows = append(rows, g)
	}
	slices.SortFunc(rows, func(a, b *protoapi.VulnerabilitySummary) int {
		var order int
		asc := req.OrderBy.GetDirection() == protoapi.VulnerabilityOrderBy_DIRECTION_ASC
		switch req.OrderBy.GetField() {
		case protoapi.VulnerabilityOrderBy_FIELD_VULNERABILITY_ID:
			order = strings.Compare(a.VulnerabilityId, b.VulnerabilityId)
			if !asc {
				order = -order
			}
		case protoapi.VulnerabilityOrderBy_FIELD_AFFECTED_WORKLOAD_COUNT:
			order = cmp.Compare(a.AffectedWorkloadCount, b.AffectedWorkloadCount)
			if !asc {
				order = -order
			}
		default:
			order = compareSeverity(a.Severity, b.Severity, asc)
		}
		return cmp.Or(order, strings.Compare(a.VulnerabilityId, b.VulnerabilityId))
	})
	total := int64(len(rows))
	rows, cursors, next, info, err := paginate(rows, req.PageSize, req.PageToken, req.PageDirection)
	return &protoapi.ListVulnerabilitiesResponse{Vulnerabilities: rows, Cursors: cursors, NextPageToken: next, TotalCount: total, PageInfo: info}, err
}

func (c *Client) CountFindings(ctx context.Context, req *protoapi.CountFindingsRequest, _ ...grpc.CallOption) (*protoapi.CountFindingsResponse, error) {
	_, findings, err := c.data(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64)
	for _, f := range filterFindings(findings, req.Filter, req.WorkloadSelector) {
		var key string
		switch req.GroupBy {
		case protoapi.CountFindingsRequest_GROUP_BY_WORKLOAD:
			key = workloadKey(f.Workload)
		case protoapi.CountFindingsRequest_GROUP_BY_TEAM:
			key = f.Workload.Team
		case protoapi.CountFindingsRequest_GROUP_BY_IMAGE:
			key = f.Image.Name + "@" + f.Image.Digest
		case protoapi.CountFindingsRequest_GROUP_BY_VULNERABILITY:
			key = f.VulnerabilityId
		default:
			key = strings.TrimPrefix(f.Severity.String(), "SEVERITY_")
		}
		counts[key]++
	}
	resp := &protoapi.CountFindingsResponse{}
	for key, count := range counts {
		resp.Buckets = append(resp.Buckets, &protoapi.CountFindingsResponse_Bucket{Key: key, Count: count})
	}
	slices.SortFunc(resp.Buckets, func(a, b *protoapi.CountFindingsResponse_Bucket) int { return strings.Compare(a.Key, b.Key) })
	return resp, nil
}

func (c *Client) GetSummary(ctx context.Context, req *protoapi.GetSummaryRequest, _ ...grpc.CallOption) (*protoapi.GetSummaryResponse, error) {
	workloads, findings, err := c.data(ctx)
	if err != nil {
		return nil, err
	}
	selector := &protoapi.WorkloadSelector{}
	coverage := &protoapi.WorkloadCoverage{}
	for _, w := range workloads {
		if !matchesScope(w.Environment, w.Team, w.Name, req.Environment, req.Team, req.Workload) ||
			!selected(req.WorkloadSelector, w.Environment, w.Team, w.Name) {
			continue
		}
		selector.Workloads = append(selector.Workloads, &protoapi.WorkloadRef{Environment: w.Environment, Team: w.Team, Name: w.Name})
		coverage.TotalWorkloadCount++
		switch {
		case w.Image == nil:
			coverage.UnresolvedImageCount++
		case w.Image.Status == protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM:
			coverage.NoSbomCount++
		case w.Image.Status == protoapi.ImageStatus_IMAGE_STATUS_SCANNED:
			coverage.ScannedCount++
		default:
			coverage.PendingCount++
		}
	}
	resp := summarize(filterFindings(findings, req.FindingFilter, selector))
	resp.Coverage = coverage
	return resp, nil
}

func (c *Client) GetVulnerability(ctx context.Context, req *protoapi.GetVulnerabilityRequest, _ ...grpc.CallOption) (*protoapi.GetVulnerabilityResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, f := range sampleFindings() {
		if matchesVulnerability(f.VulnerabilityId, req.Id) {
			return &protoapi.GetVulnerabilityResponse{Vulnerability: &protoapi.Vulnerability{
				Id: f.VulnerabilityId, Aliases: []string{alias(f.VulnerabilityId)},
				Title:       "Example " + strings.ToLower(strings.TrimPrefix(f.Severity.String(), "SEVERITY_")) + " vulnerability (fake data)",
				Description: "Synthetic vulnerability for local development, not a real advisory.",
				Epss:        f.Epss, KnownExploited: f.KnownExploited,
			}}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "vulnerability not found")
}

func (*Client) SearchComponents(context.Context, *protoapi.SearchComponentsRequest, ...grpc.CallOption) (*protoapi.SearchComponentsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "component search is not used by Nais API")
}

func matchesScope(env, team, name, wantEnv, wantTeam, wantName string) bool {
	return (wantEnv == "" || env == wantEnv) && (wantTeam == "" || team == wantTeam) && (wantName == "" || name == wantName)
}

func selected(selector *protoapi.WorkloadSelector, env, team, name string) bool {
	return selector == nil || slices.ContainsFunc(selector.Workloads, func(w *protoapi.WorkloadRef) bool {
		return w.GetEnvironment() == env && w.GetTeam() == team && w.GetName() == name
	})
}

func workloadKey(w *protoapi.WorkloadRef) string {
	return w.GetEnvironment() + "/" + w.GetTeam() + "/" + w.GetName()
}

func filterFindings(findings []*protoapi.Finding, filter *protoapi.FindingFilter, selector *protoapi.WorkloadSelector) []*protoapi.Finding {
	return slices.DeleteFunc(slices.Clone(findings), func(f *protoapi.Finding) bool {
		if !matchesScope(f.Workload.Environment, f.Workload.Team, f.Workload.Name, filter.GetEnvironment(), filter.GetTeam(), filter.GetWorkload()) ||
			!selected(selector, f.Workload.Environment, f.Workload.Team, f.Workload.Name) ||
			(filter.GetImageName() != "" && filter.GetImageName() != f.Image.Name) ||
			(filter.GetImageDigest() != "" && filter.GetImageDigest() != f.Image.Digest) ||
			(filter.GetPackage() != "" && filter.GetPackage() != f.PackageKey && filter.GetPackage() != f.Purl) ||
			(filter.GetPackageType() != "" && !strings.EqualFold(filter.GetPackageType(), packageType(f.PackageKey))) ||
			(len(filter.GetSeverities()) > 0 && !slices.Contains(filter.Severities, f.Severity)) ||
			(filter.GetHasFix() && f.FixedVersion == "") ||
			(filter.GetOnlyKnownExploited() && !f.KnownExploited) {
			return true
		}
		if len(filter.GetVulnerabilityIds()) > 0 && !slices.ContainsFunc(filter.VulnerabilityIds, func(id string) bool {
			return matchesVulnerability(f.VulnerabilityId, id)
		}) {
			return true
		}
		switch filter.GetSuppressed() {
		case protoapi.SuppressedFilter_SUPPRESSED_FILTER_INCLUDE:
			return false
		case protoapi.SuppressedFilter_SUPPRESSED_FILTER_ONLY:
			return f.Suppression == nil
		default:
			return f.Suppression != nil
		}
	})
}

func summarize(findings []*protoapi.Finding) *protoapi.GetSummaryResponse {
	resp := &protoapi.GetSummaryResponse{FindingCount: int64(len(findings)), SeverityCounts: &protoapi.VulnerabilitySeverityCounts{}}
	severities := make(map[string]protoapi.Severity)
	workloads, kevWorkloads, kev := make(map[string]bool), make(map[string]bool), make(map[string]bool)
	for _, f := range findings {
		severities[f.VulnerabilityId] = max(severities[f.VulnerabilityId], f.Severity)
		workloads[workloadKey(f.Workload)] = true
		if f.KnownExploited {
			kev[f.VulnerabilityId], kevWorkloads[workloadKey(f.Workload)] = true, true
		}
	}
	resp.UniqueVulnerabilityCount, resp.AffectedWorkloadCount = int64(len(severities)), int64(len(workloads))
	resp.UniqueKevVulnerabilityCount, resp.KevAffectedWorkloadCount = int64(len(kev)), int64(len(kevWorkloads))
	for _, severity := range severities {
		switch severity {
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
	return resp
}

func compareSeverity(a, b protoapi.Severity, asc bool) int {
	if a == b {
		return 0
	}
	order := cmp.Compare(a, b)
	if !asc {
		order = -order
	}
	return order
}

func paginate[T any](rows []T, size int32, token string, direction protoapi.PageDirection) ([]T, []string, string, *protoapi.PageInfo, error) {
	start, end := 0, len(rows)
	if direction != protoapi.PageDirection_PAGE_DIRECTION_UNSPECIFIED &&
		direction != protoapi.PageDirection_PAGE_DIRECTION_FORWARD && direction != protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
		return nil, nil, "", nil, status.Error(codes.InvalidArgument, "invalid page direction")
	}
	if token != "" {
		boundary, err := strconv.Atoi(token)
		if err != nil || boundary < 0 || (boundary == 0 && direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD) {
			return nil, nil, "", nil, status.Error(codes.InvalidArgument, "invalid fake v0s cursor")
		}
		if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
			end = min(boundary-1, len(rows))
		} else {
			start = min(boundary, len(rows))
		}
	}
	if size < 0 {
		return nil, nil, "", nil, status.Error(codes.InvalidArgument, "page size must not be negative")
	}
	if size == 0 {
		size = 100
	}
	if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
		start = max(0, end-int(size))
	} else {
		end = start + min(int(size), len(rows)-start)
	}
	cursors := make([]string, end-start)
	for i := range cursors {
		cursors[i] = strconv.Itoa(start + i + 1)
	}
	next := ""
	if end < len(rows) && len(cursors) > 0 {
		next = strconv.Itoa(end)
	}
	return rows[start:end], cursors, next, &protoapi.PageInfo{HasNextPage: end < len(rows), HasPreviousPage: start > 0}, nil
}

func clone[T proto.Message](value T) T {
	return proto.Clone(value).(T)
}
