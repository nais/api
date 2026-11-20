package v0s

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/v0s/protoapi"
	"github.com/nais/api/internal/workload"
	"google.golang.org/protobuf/proto"
)

func requireRead(ctx context.Context) error {
	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return apierror.ErrForbidden
	}
	return nil
}

func normalizeScope(input *VulnerabilityScopeInput) (scope, error) {
	s := scope{}
	if input == nil {
		return s, nil
	}
	if input.Team != nil {
		if err := input.Team.Validate(); err != nil {
			return s, err
		}
		s.team = input.Team.String()
	}
	if input.Environment != nil {
		if strings.TrimSpace(*input.Environment) == "" {
			return s, apierror.Errorf("environment cannot be empty")
		}
		s.environment = *input.Environment
	}
	if input.Workload != nil {
		if strings.TrimSpace(*input.Workload) == "" {
			return s, apierror.Errorf("workload cannot be empty")
		}
		s.workload = *input.Workload
		if s.team == "" || s.environment == "" {
			return s, apierror.Errorf("workload requires team and environment")
		}
	}
	return s, nil
}

func normalizeSelectionScope(input, filterScope *VulnerabilityScopeInput) (scope, error) {
	if filterScope != nil {
		if input != nil {
			return scope{}, apierror.Errorf("scope cannot be provided on a workload field")
		}
		input = filterScope
	}
	return normalizeScope(input)
}

func WorkloadScope(w workload.Workload) *VulnerabilityScopeInput {
	return &VulnerabilityScopeInput{
		Team: new(w.GetTeamSlug()), Environment: new(w.GetEnvironmentName()), Workload: new(w.GetName()),
	}
}

func findingFilter(s scope, input *VulnerabilityFindingFilter) (*protoapi.FindingFilter, error) {
	f := &protoapi.FindingFilter{
		Team: s.team, Environment: s.environment, Workload: s.workload,
		Suppressed: protoapi.SuppressedFilter_SUPPRESSED_FILTER_EXCLUDE,
	}
	if input == nil {
		return f, nil
	}
	if input.VulnerabilityIdentifier != nil {
		if strings.TrimSpace(*input.VulnerabilityIdentifier) == "" {
			return nil, apierror.Errorf("vulnerabilityIdentifier cannot be empty")
		}
		f.VulnerabilityId = *input.VulnerabilityIdentifier
	}
	if input.Package != nil {
		if strings.TrimSpace(*input.Package) == "" {
			return nil, apierror.Errorf("package cannot be empty")
		}
		f.Package = *input.Package
	}
	if input.OnlyWithFix != nil {
		f.HasFix = *input.OnlyWithFix
	}
	if input.OnlyKnownExploited != nil {
		f.OnlyKnownExploited = *input.OnlyKnownExploited
	}
	if input.Suppressed != nil {
		switch *input.Suppressed {
		case SuppressedFilterExclude:
		case SuppressedFilterInclude:
			f.Suppressed = protoapi.SuppressedFilter_SUPPRESSED_FILTER_INCLUDE
		case SuppressedFilterOnly:
			f.Suppressed = protoapi.SuppressedFilter_SUPPRESSED_FILTER_ONLY
		default:
			return nil, apierror.Errorf("invalid suppressed filter")
		}
	}
	for _, value := range input.Severities {
		number, ok := protoapi.Severity_value["SEVERITY_"+string(value)]
		if !ok || number == 0 {
			return nil, apierror.Errorf("invalid severity")
		}
		f.Severities = append(f.Severities, protoapi.Severity(number))
	}
	slices.Sort(f.Severities)
	f.Severities = slices.Compact(f.Severities)
	return f, nil
}

func Findings(ctx context.Context, input *VulnerabilityScopeInput, filter *VulnerabilityFindingFilter, first *int, after *pagination.Cursor) (*VulnerabilityFindingConnection, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	var filterScope *VulnerabilityScopeInput
	if filter != nil {
		filterScope = filter.Scope
	}
	s, err := normalizeSelectionScope(input, filterScope)
	if err != nil {
		return nil, err
	}
	f, err := findingFilter(s, filter)
	if err != nil {
		return nil, err
	}
	data, err := proto.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("encode v0s finding filter: %w", err)
	}
	p, err := parsePage(first, after, selectionKey("findings", string(data)))
	if err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	current, err := l.currentWorkloads(ctx, s)
	if err != nil {
		return nil, err
	}
	edges, info, err := readPage(ctx, p,
		func(ctx context.Context, size int32, token string) ([]*protoapi.Finding, []string, string, error) {
			resp, err := l.manager.Vulnerabilities.ListFindings(ctx, &protoapi.ListFindingsRequest{Filter: f, PageSize: size, PageToken: token})
			if err != nil {
				return nil, nil, "", backendError("list v0s findings", err)
			}
			return resp.GetFindings(), resp.GetCursors(), resp.GetNextPageToken(), nil
		}, func(f *protoapi.Finding) bool {
			w := f.GetWorkload()
			active := current[workloadKey{w.GetEnvironment(), w.GetTeam(), w.GetName()}]
			return active.GetImage() != nil && f.GetImage() != nil &&
				active.GetImage().GetName() == f.GetImage().GetName() &&
				active.GetImage().GetDigest() == f.GetImage().GetDigest()
		})
	if err != nil {
		return nil, err
	}
	info.CountLoader = func(ctx context.Context) (int, error) {
		full := proto.Clone(f).(*protoapi.FindingFilter)
		full.Workload = ""
		count, err := l.countCurrent(ctx, full, current)
		return int(count), err
	}
	result := &VulnerabilityFindingConnection{Edges: make([]VulnerabilityFindingEdge, 0, len(edges)), PageInfo: info}
	for _, edge := range edges {
		value, err := finding(edge.Node)
		if err != nil {
			return nil, err
		}
		result.Edges = append(result.Edges, VulnerabilityFindingEdge{Node: value, Cursor: edge.Cursor})
	}
	return result, nil
}

func FindingSummary(ctx context.Context, input *VulnerabilityScopeInput, filter *VulnerabilityFindingFilter) (*VulnerabilityFindingSummary, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	var filterScope *VulnerabilityScopeInput
	if filter != nil {
		filterScope = filter.Scope
	}
	s, err := normalizeSelectionScope(input, filterScope)
	if err != nil {
		return nil, err
	}
	f, err := findingFilter(s, filter)
	if err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	current, err := l.currentWorkloads(ctx, s)
	if err != nil {
		return nil, err
	}
	// Grouped counts can be reused across workload fields in a team/environment.
	// Reconciliation discards buckets for removed workloads and obsolete images.
	f.Workload = ""
	filters := []*protoapi.FindingFilter{f}
	for _, sev := range []protoapi.Severity{
		protoapi.Severity_SEVERITY_UNKNOWN, protoapi.Severity_SEVERITY_LOW, protoapi.Severity_SEVERITY_MEDIUM,
		protoapi.Severity_SEVERITY_HIGH, protoapi.Severity_SEVERITY_CRITICAL,
	} {
		clone := proto.Clone(f).(*protoapi.FindingFilter)
		if len(f.Severities) == 0 || slices.Contains(f.Severities, sev) {
			clone.Severities = []protoapi.Severity{sev}
		} else {
			filters = append(filters, nil)
			continue
		}
		filters = append(filters, clone)
	}
	fixed := proto.Clone(f).(*protoapi.FindingFilter)
	fixed.HasFix = true
	known := proto.Clone(f).(*protoapi.FindingFilter)
	known.OnlyKnownExploited = true
	filters = append(filters, fixed, known)
	counts := make([]int64, len(filters))
	for i, filter := range filters {
		if filter == nil {
			continue
		}
		counts[i], err = l.countCurrent(ctx, filter, current)
		if err != nil {
			return nil, err
		}
	}
	return &VulnerabilityFindingSummary{
		TotalFindings: counts[0], CountsBySeverity: &VulnerabilitySeverityCounts{
			Unknown: counts[1], Low: counts[2], Medium: counts[3], High: counts[4], Critical: counts[5],
		},
		FixableFindings: counts[6], KnownExploitedFindings: counts[7],
	}, nil
}

func Coverage(ctx context.Context, input *VulnerabilityScopeInput) (*VulnerabilityCoverageCounts, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	s, err := normalizeScope(input)
	if err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	current, err := l.currentWorkloads(ctx, s)
	if err != nil {
		return nil, err
	}
	inventory, err := l.inventory.Load(ctx, struct{}{})
	if err != nil {
		return nil, err
	}
	result := &VulnerabilityCoverageCounts{}
	for key := range inventory {
		if !s.matches(key) {
			continue
		}
		value, err := coverage(current[key])
		if err != nil {
			return nil, err
		}
		result.TotalWorkloads++
		switch value.Status {
		case VulnerabilityCoverageStatusScanned:
			result.Scanned++
		case VulnerabilityCoverageStatusNoSbom:
			result.NoSbom++
		case VulnerabilityCoverageStatusFailed:
			result.Failed++
		case VulnerabilityCoverageStatusUnresolvedImage:
			result.UnresolvedImage++
		default:
			result.Pending++
		}
	}
	return result, nil
}

func WorkloadCoverage(ctx context.Context, w workload.Workload) (*VulnerabilityCoverage, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	s, err := normalizeScope(WorkloadScope(w))
	if err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	current, err := l.currentWorkloads(ctx, s)
	if err != nil {
		return nil, err
	}
	return coverage(current[workloadKey{s.environment, s.team, s.workload}])
}

func VulnerabilityByIdentifier(ctx context.Context, identifier string) (*Vulnerability, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(identifier) == "" {
		return nil, apierror.Errorf("identifier cannot be empty")
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	return l.advisories.Load(ctx, identifier)
}

func Suppressions(ctx context.Context, input *VulnerabilityScopeInput, filter *VulnerabilitySuppressionFilter, first *int, after *pagination.Cursor) (*VulnerabilitySuppressionConnection, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	var filterScope *VulnerabilityScopeInput
	if filter != nil {
		filterScope = filter.Scope
	}
	s, err := normalizeSelectionScope(input, filterScope)
	if err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	req := &protoapi.ListSuppressionsRequest{Team: s.team, Environment: s.environment, Workload: s.workload}
	if filter != nil {
		if filter.VulnerabilityIdentifier != nil {
			if strings.TrimSpace(*filter.VulnerabilityIdentifier) == "" {
				return nil, apierror.Errorf("vulnerabilityIdentifier cannot be empty")
			}
			req.VulnerabilityId = *filter.VulnerabilityIdentifier
		}
		if filter.ImageName != nil {
			if strings.TrimSpace(*filter.ImageName) == "" {
				return nil, apierror.Errorf("imageName cannot be empty")
			}
			req.ImageName = *filter.ImageName
		}
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode v0s suppression filter: %w", err)
	}
	p, err := parsePage(first, after, selectionKey("suppressions", string(data)))
	if err != nil {
		return nil, err
	}
	edges, info, err := readPage(ctx, p,
		func(ctx context.Context, size int32, token string) ([]*protoapi.Suppression, []string, string, error) {
			req.PageSize, req.PageToken = size, token
			resp, err := l.manager.Suppressions.ListSuppressions(ctx, req)
			if err != nil {
				return nil, nil, "", backendError("list v0s suppressions", err)
			}
			return resp.GetSuppressions(), resp.GetCursors(), resp.GetNextPageToken(), nil
		}, func(*protoapi.Suppression) bool { return true })
	if err != nil {
		return nil, err
	}
	info.CountLoader = func(ctx context.Context) (int, error) {
		return l.suppressionCounts.Load(ctx, string(data))
	}
	result := &VulnerabilitySuppressionConnection{Edges: make([]VulnerabilitySuppressionEdge, 0, len(edges)), PageInfo: info}
	for _, edge := range edges {
		value, err := suppression(edge.Node)
		if err != nil {
			return nil, err
		}
		result.Edges = append(result.Edges, VulnerabilitySuppressionEdge{Node: value, Cursor: edge.Cursor})
	}
	return result, nil
}
