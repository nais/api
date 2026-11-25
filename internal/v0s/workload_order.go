package v0s

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/workload"
)

func ValidateWorkloadFilter(filter *workload.TeamWorkloadsFilter) error {
	if filter != nil && (len(filter.States) > 0 || len(filter.WorkloadStatusErrorTypes) > 0 ||
		filter.SbomStatus != nil || len(filter.VulnerabilityCoverageStatuses) > 0) {
		return apierror.Errorf("Only environments and labels are supported for vulnerability workload selection")
	}
	return nil
}

func uniqueSeverityCount(field workload.WorkloadOrderField, counts *VulnerabilitySeverityCounts) (int64, error) {
	switch field {
	case workload.WorkloadOrderFieldVulnerabilityUniqueCritical:
		return counts.Critical, nil
	case workload.WorkloadOrderFieldVulnerabilityUniqueHigh:
		return counts.High, nil
	case workload.WorkloadOrderFieldVulnerabilityUniqueMedium:
		return counts.Medium, nil
	case workload.WorkloadOrderFieldVulnerabilityUniqueLow:
		return counts.Low, nil
	case workload.WorkloadOrderFieldVulnerabilityUniqueUnknown:
		return counts.Unknown, nil
	default:
		return 0, apierror.Errorf("Invalid unique-vulnerability workload order")
	}
}

func SortWorkloads(ctx context.Context, items []workload.Workload, filter *VulnerabilityFindingFilter, order *workload.WorkloadOrder) ([]VulnerabilityCoverageStatus, error) {
	if order == nil || !order.Field.IsUniqueVulnerabilityOrder() ||
		(order.Direction != model.OrderDirectionAsc && order.Direction != model.OrderDirectionDesc) {
		return nil, apierror.Errorf("Invalid unique-vulnerability workload order")
	}
	overviews, err := WorkloadOverviews(ctx, items, filter)
	if err != nil {
		return nil, err
	}
	type sortable struct {
		item   workload.Workload
		status VulnerabilityCoverageStatus
		count  int64
	}
	values := make([]sortable, len(items))
	for i, value := range overviews {
		count, err := uniqueSeverityCount(order.Field, value.CountsBySeverity)
		if err != nil {
			return nil, err
		}
		values[i] = sortable{item: items[i], status: value.Coverage.Status, count: count}
	}
	slices.SortFunc(values, func(a, b sortable) int {
		aScanned, bScanned := a.status == VulnerabilityCoverageStatusScanned, b.status == VulnerabilityCoverageStatusScanned
		if aScanned != bScanned {
			if aScanned {
				return 1
			}
			return -1
		}
		if aScanned {
			compared := cmp.Compare(a.count, b.count)
			if order.Direction == model.OrderDirectionDesc {
				compared = -compared
			}
			if compared != 0 {
				return compared
			}
		}
		return cmp.Or(cmp.Compare(a.item.GetName(), b.item.GetName()),
			cmp.Compare(a.item.GetEnvironmentName(), b.item.GetEnvironmentName()),
			cmp.Compare(a.item.GetType().String(), b.item.GetType().String()))
	})
	statuses := make([]VulnerabilityCoverageStatus, len(items))
	for i, value := range values {
		items[i], statuses[i] = value.item, value.status
	}
	return statuses, nil
}

type workloadPageCursor struct {
	Version   int    `json:"version"`
	Selection string `json:"selection"`
	Offset    int    `json:"offset"`
}

func compareLabelFilters(a, b *model.LabelFilter) int {
	if compared := cmp.Compare(a.Key, b.Key); compared != 0 {
		return compared
	}
	if a.Value == nil {
		if b.Value == nil {
			return 0
		}
		return -1
	}
	if b.Value == nil {
		return 1
	}
	return cmp.Compare(*a.Value, *b.Value)
}

func ParseWorkloadPage(scope Scope, filter *workload.TeamWorkloadsFilter, findings *VulnerabilityFindingFilter, order *workload.WorkloadOrder, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Pagination, string, error) {
	if err := ValidateWorkloadFilter(filter); err != nil {
		return nil, "", err
	}
	if order == nil || !order.Field.IsUniqueVulnerabilityOrder() ||
		(order.Direction != model.OrderDirectionAsc && order.Direction != model.OrderDirectionDesc) {
		return nil, "", apierror.Errorf("Invalid unique-vulnerability workload order")
	}
	encoded, err := workloadOverviewFilter(findings)
	if err != nil {
		return nil, "", err
	}
	environments := []string{}
	labels := []*model.LabelFilter{}
	if filter != nil {
		environments = append(environments, filter.Environments...)
		for _, label := range filter.Labels {
			if label != nil && label.Key != "" {
				labels = append(labels, label)
			}
		}
	}
	slices.Sort(environments)
	environments = slices.Compact(environments)
	slices.SortFunc(labels, compareLabelFilters)
	labels = slices.CompactFunc(labels, func(a, b *model.LabelFilter) bool { return compareLabelFilters(a, b) == 0 })
	data, err := json.Marshal(struct {
		Scope        Scope
		Environments []string
		Labels       []*model.LabelFilter
		Findings     []byte
		Order        *workload.WorkloadOrder
	}{scope, environments, labels, []byte(encoded), order})
	if err != nil {
		return nil, "", fmt.Errorf("encode workload vulnerability selection: %w", err)
	}
	binding := selectionKey("workload-unique-vulnerabilities", string(data))
	decode := func(input *pagination.Cursor) (*pagination.Cursor, error) {
		if input == nil {
			return nil, nil
		}
		data, err := base64.RawURLEncoding.DecodeString(input.Keyset)
		var cursor workloadPageCursor
		if input.Keyset == "" || err != nil || json.Unmarshal(data, &cursor) != nil ||
			cursor.Version != 1 || cursor.Selection != binding || cursor.Offset < 0 || cursor.Offset >= math.MaxInt32 {
			return nil, apierror.Errorf("Cursor does not belong to this workload vulnerability selection; restart without a cursor")
		}
		return &pagination.Cursor{Offset: cursor.Offset}, nil
	}
	decodedAfter, err := decode(after)
	if err != nil {
		return nil, "", err
	}
	decodedBefore, err := decode(before)
	if err != nil {
		return nil, "", err
	}
	page, err := pagination.ParsePage(first, decodedAfter, last, decodedBefore)
	return page, binding, err
}

func BindWorkloadCursors(connection *pagination.Connection[workload.Workload], binding string) error {
	for i := range connection.Edges {
		cursor := &connection.Edges[i].Cursor
		data, err := json.Marshal(workloadPageCursor{Version: 1, Selection: binding, Offset: cursor.Offset})
		if err != nil {
			return fmt.Errorf("encode workload vulnerability cursor: %w", err)
		}
		cursor.Keyset = base64.RawURLEncoding.EncodeToString(data)
	}
	if len(connection.Edges) > 0 {
		connection.PageInfo.StartCursor = &connection.Edges[0].Cursor
		connection.PageInfo.EndCursor = &connection.Edges[len(connection.Edges)-1].Cursor
	}
	return nil
}
