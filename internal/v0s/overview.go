package v0s

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/protobuf/proto"
)

func vulnerabilityOverviewOrder(input *VulnerabilityOverviewOrder) (*protoapi.VulnerabilityOrderBy, error) {
	if input == nil {
		input = &VulnerabilityOverviewOrder{Field: VulnerabilityOverviewOrderFieldSeverity, Direction: model.OrderDirectionDesc}
	}
	order := &protoapi.VulnerabilityOrderBy{}
	switch input.Field {
	case VulnerabilityOverviewOrderFieldSeverity:
		order.Field = protoapi.VulnerabilityOrderBy_FIELD_SEVERITY
	case VulnerabilityOverviewOrderFieldIdentifier:
		order.Field = protoapi.VulnerabilityOrderBy_FIELD_VULNERABILITY_ID
	case VulnerabilityOverviewOrderFieldAffectedWorkloadCount:
		order.Field = protoapi.VulnerabilityOrderBy_FIELD_AFFECTED_WORKLOAD_COUNT
	default:
		return nil, apierror.Errorf("invalid vulnerability overview order field")
	}
	switch input.Direction {
	case model.OrderDirectionAsc:
		order.Direction = protoapi.VulnerabilityOrderBy_DIRECTION_ASC
	case model.OrderDirectionDesc:
		order.Direction = protoapi.VulnerabilityOrderBy_DIRECTION_DESC
	default:
		return nil, apierror.Errorf("invalid vulnerability overview order direction")
	}
	return order, nil
}

func vulnerabilityOverviews(ctx context.Context, l *loaders, filter *protoapi.FindingFilter, selector *protoapi.WorkloadSelector, first *int, after *pagination.Cursor, orderBy *VulnerabilityOverviewOrder) (*VulnerabilityOverviewConnection, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	order, err := vulnerabilityOverviewOrder(orderBy)
	if err != nil {
		return nil, err
	}
	req := &protoapi.ListVulnerabilitiesRequest{Filter: filter, WorkloadSelector: selector, OrderBy: order}
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode v0s vulnerability overview selection: %w", err)
	}
	p, err := parsePage(first, after, selectionKey("vulnerability-overview", string(data)))
	if err != nil {
		return nil, err
	}
	var total int64
	edges, info, err := readPage(ctx, p,
		func(ctx context.Context, size int32, token string) ([]*protoapi.VulnerabilitySummary, []string, string, error) {
			req.PageSize, req.PageToken = size, token
			resp, err := l.manager.Vulnerabilities.ListVulnerabilities(ctx, req)
			if err != nil {
				return nil, nil, "", backendError("list v0s vulnerability overview", err)
			}
			total = resp.GetTotalCount()
			return resp.GetVulnerabilities(), resp.GetCursors(), resp.GetNextPageToken(), nil
		}, func(*protoapi.VulnerabilitySummary) bool { return true })
	if err != nil {
		return nil, err
	}
	info.TotalCount = int(total)
	result := &VulnerabilityOverviewConnection{Edges: make([]VulnerabilityOverviewEdge, 0, len(edges)), PageInfo: info}
	for _, edge := range edges {
		value := edge.Node
		if value.GetVulnerabilityId() == "" {
			return nil, fmt.Errorf("v0s vulnerability overview is missing an identifier")
		}
		result.Edges = append(result.Edges, VulnerabilityOverviewEdge{
			Cursor: edge.Cursor,
			Node: &VulnerabilityOverview{
				VulnerabilityIdentifier: value.GetVulnerabilityId(), Severity: severity(value.GetSeverity()),
				FindingCount: value.GetFindingCount(), AffectedWorkloadCount: value.GetAffectedWorkloadCount(),
			},
		})
	}
	return result, nil
}

func WorkloadOverview(ctx context.Context, input Scope, filter *VulnerabilityFindingFilter) (*VulnerabilityWorkloadOverview, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	f, err := findingFilter(scope{}, filter)
	if err != nil {
		return nil, err
	}
	data, err := proto.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("encode v0s workload overview filter: %w", err)
	}
	s := input.selection()
	return l.workloadOverviews.Load(ctx, workloadOverviewKey{
		workload: workloadKey{s.environment, s.team, s.workload}, filter: string(data),
	})
}

type workloadOverviewKey struct {
	workload workloadKey
	filter   string
}

func (l *loaders) loadWorkloadOverviews(ctx context.Context, keys []workloadOverviewKey) ([]*VulnerabilityWorkloadOverview, []error) {
	values := make([]*VulnerabilityWorkloadOverview, len(keys))
	errs := make([]error, len(keys))
	inventory, err := l.inventory.Load(ctx, struct{}{})
	if err != nil {
		for i := range keys {
			errs[i] = err
		}
		return values, errs
	}
	groups := make(map[string][]int)
	for i, key := range keys {
		groups[key.filter] = append(groups[key.filter], i)
	}
	for filter, indices := range groups {
		selected := make([]workloadKey, 0, len(indices))
		for _, i := range indices {
			if _, exists := inventory[keys[i].workload]; exists {
				selected = append(selected, keys[i].workload)
			}
		}
		rows, err := l.fetchWorkloadOverviews(ctx, filter, selected, inventory)
		for _, i := range indices {
			if err != nil {
				errs[i] = err
				continue
			}
			value := rows[keys[i].workload]
			if value == nil {
				value = &VulnerabilityWorkloadOverview{
					CountsBySeverity: &VulnerabilitySeverityCounts{},
					Coverage:         &VulnerabilityCoverage{Status: VulnerabilityCoverageStatusPending},
				}
			}
			values[i] = value
		}
	}
	return values, errs
}

func (l *loaders) fetchWorkloadOverviews(ctx context.Context, encodedFilter string, selected []workloadKey, inventory map[workloadKey]string) (map[workloadKey]*VulnerabilityWorkloadOverview, error) {
	filter := &protoapi.FindingFilter{}
	if err := proto.Unmarshal([]byte(encodedFilter), filter); err != nil {
		return nil, fmt.Errorf("decode v0s workload overview filter: %w", err)
	}
	selector := &protoapi.WorkloadSelector{}
	for _, key := range selected {
		selector.Workloads = append(selector.Workloads, &protoapi.WorkloadRef{
			Environment: key.environment, Team: key.team, Name: key.name,
		})
	}
	req := &protoapi.ListWorkloadsRequest{
		WorkloadSelector: selector, PageSize: 1000,
		Overview: &protoapi.WorkloadOverviewOptions{FindingFilter: filter},
	}
	result := make(map[workloadKey]*VulnerabilityWorkloadOverview)
	for {
		resp, err := l.manager.Vulnerabilities.ListWorkloads(ctx, req)
		if err != nil {
			return nil, backendError("load v0s workload overviews", err)
		}
		if resp == nil || resp.TotalCount == nil {
			return nil, fmt.Errorf("v0s workload overview is not available")
		}
		for _, w := range resp.GetWorkloads() {
			key := workloadKey{w.GetEnvironment(), w.GetTeam(), w.GetName()}
			image, exists := inventory[key]
			if !exists || image != w.GetImageReference() {
				continue
			}
			counts := w.GetFindingCounts()
			if counts == nil || counts.GetSeverityCounts() == nil {
				return nil, fmt.Errorf("v0s workload overview is missing counts")
			}
			scan, err := coverage(w)
			if err != nil {
				return nil, err
			}
			severities := counts.GetSeverityCounts()
			result[key] = &VulnerabilityWorkloadOverview{
				FindingCount: counts.GetFindingCount(), UniqueVulnerabilityCount: counts.GetUniqueVulnerabilityCount(),
				KnownExploitedVulnerabilityCount: counts.GetUniqueKevVulnerabilityCount(),
				CountsBySeverity: &VulnerabilitySeverityCounts{
					Critical: severities.GetCritical(), High: severities.GetHigh(), Medium: severities.GetMedium(),
					Low: severities.GetLow(), Unknown: severities.GetUnknown(),
				},
				Coverage: scan,
			}
		}
		next := resp.GetNextPageToken()
		if next == "" {
			return result, nil
		}
		if next == req.PageToken {
			return nil, fmt.Errorf("v0s returned a non-advancing workload overview cursor")
		}
		req.PageToken = next
	}
}
