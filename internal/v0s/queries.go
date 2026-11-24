package v0s

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/workload"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/protobuf/proto"
)

func requireRead(ctx context.Context) error {
	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return apierror.ErrForbidden
	}
	return nil
}

func (s Scope) selection() scope {
	return scope{team: s.Team.String(), environment: s.Environment, workload: s.Workload}
}

func WorkloadScope(w workload.Workload) Scope {
	return Scope{
		Team: w.GetTeamSlug(), Environment: w.GetEnvironmentName(), Workload: w.GetName(),
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
	for _, id := range input.VulnerabilityIdentifiers {
		if strings.TrimSpace(id) == "" {
			return nil, apierror.Errorf("vulnerabilityIdentifiers cannot contain empty values")
		}
	}
	f.VulnerabilityIds = slices.Clone(input.VulnerabilityIdentifiers)
	slices.Sort(f.VulnerabilityIds)
	f.VulnerabilityIds = slices.Compact(f.VulnerabilityIds)
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

func Findings(ctx context.Context, input Scope, filter *VulnerabilityFindingFilter, first *int, after *pagination.Cursor) (*VulnerabilityFindingConnection, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	s := input.selection()
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
	grouped := proto.Clone(f).(*protoapi.FindingFilter)
	grouped.Workload = ""
	info.CountLoader = func(ctx context.Context) (int, error) {
		count, err := l.countCurrent(ctx, grouped, current)
		return int(count), err
	}
	result := &VulnerabilityFindingConnection{
		Connection: pagination.Connection[*VulnerabilityFinding]{Edges: make([]VulnerabilityFindingEdge, 0, len(edges)), PageInfo: info},
		loadFacets: func(ctx context.Context) (*VulnerabilityFindingFacets, error) {
			return findingFacets(ctx, l, grouped, current)
		},
	}
	for _, edge := range edges {
		value, err := finding(edge.Node)
		if err != nil {
			return nil, err
		}
		result.Edges = append(result.Edges, VulnerabilityFindingEdge{Node: value, Cursor: edge.Cursor})
	}
	return result, nil
}

func findingFacets(ctx context.Context, l *loaders, f *protoapi.FindingFilter, current map[workloadKey]*protoapi.Workload) (*VulnerabilityFindingFacets, error) {
	// Grouped counts can be reused across workload fields in a team/environment.
	// Reconciliation discards buckets for removed workloads and obsolete images.
	filters := make([]*protoapi.FindingFilter, 0, 7)
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
		count, err := l.countCurrent(ctx, filter, current)
		if err != nil {
			return nil, err
		}
		counts[i] = count
	}
	return &VulnerabilityFindingFacets{
		CountsBySeverity: &VulnerabilitySeverityCounts{
			Unknown: counts[0], Low: counts[1], Medium: counts[2], High: counts[3], Critical: counts[4],
		},
		FixableFindings: counts[5], KnownExploitedFindings: counts[6],
	}, nil
}

func WorkloadCoverageStatuses(ctx context.Context, team Scope, workloads []workload.Workload) ([]VulnerabilityCoverageStatus, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	current, err := l.currentWorkloads(ctx, team.selection())
	if err != nil {
		return nil, err
	}
	result := make([]VulnerabilityCoverageStatus, len(workloads))
	for i, w := range workloads {
		key := workloadKey{w.GetEnvironmentName(), w.GetTeamSlug().String(), w.GetName()}
		value, err := coverage(current[key])
		if err != nil {
			return nil, err
		}
		result[i] = value.Status
	}
	return result, nil
}

func WorkloadCoverage(ctx context.Context, w workload.Workload) (*VulnerabilityCoverage, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	s := WorkloadScope(w).selection()
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

func Suppressions(ctx context.Context, input Scope, filter *VulnerabilitySuppressionFilter, first *int, after *pagination.Cursor) (*VulnerabilitySuppressionConnection, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	s := input.selection()
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
	inventory, err := l.inventory.Load(ctx, struct{}{})
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
		}, func(item *protoapi.Suppression) bool { return suppressionInInventory(inventory, item) })
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
