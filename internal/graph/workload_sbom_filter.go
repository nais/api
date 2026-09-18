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

	status, err := vulnerability.ParseSBOMStatus(*filter.SbomStatus)
	if err != nil {
		return nil, err
	}
	return vulnerability.FilterWorkloadsBySBOMStatus(ctx, workloads, status)
}
