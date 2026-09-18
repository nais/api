package graph

import (
	"context"

	"github.com/nais/api/internal/vulnerability"
	"github.com/nais/api/internal/workload"
)

func filterWorkloadsBySBOMStatus(ctx context.Context, workloads []workload.Workload, filter *workload.TeamWorkloadsFilter) ([]workload.Workload, error) {
	if filter == nil || filter.SbomStatus == nil {
		return workloads, nil
	}

	filtered := make([]workload.Workload, 0, len(workloads))
	for _, wl := range workloads {
		status, err := vulnerability.GetSbomStatus(ctx, wl.GetImageString())
		if err != nil {
			return nil, err
		}
		if status.String() == *filter.SbomStatus {
			filtered = append(filtered, wl)
		}
	}

	return filtered, nil
}
