package v0s

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/workload"
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

func vulnerabilityOverviews(ctx context.Context, l *loaders, filter *protoapi.FindingFilter, selector *protoapi.WorkloadSelector, binding string, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *VulnerabilityOverviewOrder) (*VulnerabilityOverviewConnection, error) {
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
	p, err := parsePage(first, after, last, before, selectionKey("vulnerability-overview", binding+string(data)))
	if err != nil {
		return nil, err
	}
	var total int64
	edges, info, err := readPage(ctx, p,
		func(ctx context.Context, size int32, token string, direction protoapi.PageDirection) ([]*protoapi.VulnerabilitySummary, []string, *protoapi.PageInfo, error) {
			req.PageSize, req.PageToken = size, token
			req.PageDirection = direction
			resp, err := l.manager.Vulnerabilities.ListVulnerabilities(ctx, req)
			if err != nil {
				return nil, nil, nil, backendError("list v0s vulnerability overview", err)
			}
			total = resp.GetTotalCount()
			return resp.GetVulnerabilities(), resp.GetCursors(), resp.GetPageInfo(), nil
		}, nil)
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

func WorkloadOverview(ctx context.Context, input workload.Workload, filter *VulnerabilityFindingFilter) (*VulnerabilityWorkloadOverview, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	encoded, err := workloadOverviewFilter(filter)
	if err != nil {
		return nil, err
	}
	return l.workloadOverviews.Load(ctx, overviewKey(input, encoded))
}

func workloadOverviewFilter(filter *VulnerabilityFindingFilter) (string, error) {
	f, err := findingFilter(scope{}, filter)
	if err != nil {
		return "", err
	}
	data, err := proto.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("encode v0s workload overview filter: %w", err)
	}
	return string(data), nil
}

func WorkloadOverviews(ctx context.Context, items []workload.Workload, filter *VulnerabilityFindingFilter) ([]*VulnerabilityWorkloadOverview, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	encoded, err := workloadOverviewFilter(filter)
	if err != nil {
		return nil, err
	}
	keys := make([]workloadOverviewKey, len(items))
	for i, w := range items {
		keys[i] = overviewKey(w, encoded)
	}
	return l.workloadOverviews.LoadAll(ctx, keys)
}

type workloadIdentity struct {
	workload workloadKey
	kind     workload.Type
}

type workloadOverviewKey struct {
	workloadIdentity
	image, filter string
}

func overviewKey(w workload.Workload, filter string) workloadOverviewKey {
	return workloadOverviewKey{
		workloadIdentity: workloadIdentity{
			workload: workloadKey{w.GetEnvironmentName(), w.GetTeamSlug().String(), w.GetName()},
			kind:     w.GetType(),
		},
		image: w.GetImageString(), filter: filter,
	}
}

func (l *loaders) loadWorkloadOverviews(ctx context.Context, keys []workloadOverviewKey) ([]*VulnerabilityWorkloadOverview, []error) {
	values := make([]*VulnerabilityWorkloadOverview, len(keys))
	errs := make([]error, len(keys))
	groups := make(map[string][]int)
	for i, key := range keys {
		groups[key.filter] = append(groups[key.filter], i)
	}
	for filter, indices := range groups {
		selected := make([]workloadOverviewKey, 0, len(indices))
		for _, i := range indices {
			selected = append(selected, keys[i])
		}
		rows, err := l.fetchWorkloadOverviews(ctx, filter, selected)
		for _, i := range indices {
			if err != nil {
				errs[i] = err
				continue
			}
			value := rows[keys[i]]
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

func (l *loaders) fetchWorkloadOverviews(ctx context.Context, encodedFilter string, selected []workloadOverviewKey) (map[workloadOverviewKey]*VulnerabilityWorkloadOverview, error) {
	filter := &protoapi.FindingFilter{}
	if err := proto.Unmarshal([]byte(encodedFilter), filter); err != nil {
		return nil, fmt.Errorf("decode v0s workload overview filter: %w", err)
	}
	identities := make(map[workloadKey][]workloadOverviewKey)
	refs := make(map[workloadKey]*protoapi.Workload)
	for _, key := range selected {
		identities[key.workload] = append(identities[key.workload], key)
		refs[key.workload] = nil
	}
	req := &protoapi.ListWorkloadsRequest{
		WorkloadSelector: workloadSelector(refs), PageSize: 1000,
		Overview: &protoapi.WorkloadOverviewOptions{FindingFilter: filter},
	}
	result := make(map[workloadOverviewKey]*VulnerabilityWorkloadOverview)
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
			matching := make([]workloadOverviewKey, 0, len(identities[key]))
			for _, identity := range identities[key] {
				if identity.image != "" && identity.image == w.GetImageReference() {
					matching = append(matching, identity)
				}
			}
			if len(matching) == 0 {
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
			value := &VulnerabilityWorkloadOverview{
				FindingCount: counts.GetFindingCount(), UniqueVulnerabilityCount: counts.GetUniqueVulnerabilityCount(),
				KnownExploitedVulnerabilityCount: counts.GetUniqueKevVulnerabilityCount(),
				CountsBySeverity: &VulnerabilitySeverityCounts{
					Critical: severities.GetCritical(), High: severities.GetHigh(), Medium: severities.GetMedium(),
					Low: severities.GetLow(), Unknown: severities.GetUnknown(),
				},
				Coverage: scan,
			}
			for _, identity := range matching {
				result[identity] = value
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
