package v0s

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/workload"
	protoapi "github.com/nais/v0s/api/v0s/v1"
)

func affectedWorkloadKey(w workload.Workload) string {
	return strings.Join([]string{w.GetName(), w.GetEnvironmentName(), w.GetTeamSlug().String(), w.GetType().String()}, "\x00")
}

func affectedWorkloads(ctx context.Context, scope Scope, selected []workload.Workload, current map[workloadKey]*protoapi.Workload, filter *VulnerabilityFindingFilter, binding string, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*workload.WorkloadConnection, error) {
	encoded, err := workloadOverviewFilter(filter)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(scope)
	if err != nil {
		return nil, fmt.Errorf("encode affected workload scope: %w", err)
	}
	p, err := parsePage(first, after, last, before, selectionKey("affected-workloads-name-environment-team-type-asc", binding+string(data)+encoded))
	if err != nil {
		return nil, err
	}
	overviews, err := WorkloadOverviews(ctx, selected, filter)
	if err != nil {
		return nil, err
	}
	affected := make([]workload.Workload, 0)
	seen := make(map[workloadIdentity]bool)
	for i, value := range overviews {
		key := overviewKey(selected[i], "")
		if value.FindingCount > 0 && !seen[key.workloadIdentity] {
			seen[key.workloadIdentity] = true
			affected = append(affected, selected[i])
		}
	}
	slices.SortFunc(affected, func(a, b workload.Workload) int {
		return strings.Compare(affectedWorkloadKey(a), affectedWorkloadKey(b))
	})
	edges, info, err := readPage(ctx, p, func(_ context.Context, size int32, token string, direction protoapi.PageDirection) ([]workload.Workload, []string, *protoapi.PageInfo, error) {
		start, end := 0, len(affected)
		if token != "" {
			boundary, _ := slices.BinarySearchFunc(affected, token, func(w workload.Workload, boundary string) int {
				return strings.Compare(affectedWorkloadKey(w), boundary)
			})
			if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
				end = boundary
			} else {
				start = boundary
				for start < len(affected) && affectedWorkloadKey(affected[start]) <= token {
					start++
				}
			}
		}
		if direction == protoapi.PageDirection_PAGE_DIRECTION_BACKWARD {
			start = max(0, end-int(size))
		} else {
			end = min(len(affected), start+int(size))
		}
		items := affected[start:end]
		cursors := make([]string, len(items))
		for i, w := range items {
			cursors[i] = affectedWorkloadKey(w)
		}
		return items, cursors, &protoapi.PageInfo{HasPreviousPage: start > 0, HasNextPage: end < len(affected)}, nil
	}, nil)
	if err != nil {
		return nil, err
	}
	info.TotalCount = len(affected)
	coverage, err := selectedWorkloadCoverage(selected, current)
	if err != nil {
		return nil, err
	}
	return &workload.WorkloadConnection{
		Connection: pagination.Connection[workload.Workload]{Edges: edges, PageInfo: info},
		Facets: &workload.WorkloadFacets{LoadCoverage: func(context.Context) (*workload.VulnerabilityCoverageCounts, error) {
			return coverage, nil
		}},
	}, nil
}
