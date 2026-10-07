package graph

import (
	"context"

	"github.com/nais/api/internal/vulnerability"
	"github.com/nais/api/internal/workload"
)

func filterWorkloadsBySBOMStatus(ctx context.Context, workloads []workload.Workload, filter *workload.TeamWorkloadsFilter) ([]workload.Workload, error) {
	if filter == nil || (filter.SbomStatus == nil && filter.SbomStatuses == nil && filter.HasVulnerabilityData == nil) {
		return workloads, nil
	}

	names := filter.SbomStatuses
	if filter.SbomStatus != nil {
		names = append([]string{*filter.SbomStatus}, names...)
	}
	statuses := make([]vulnerability.SBOMStatus, 0, len(names))
	for _, name := range names {
		status, err := vulnerability.ParseSBOMStatus(name)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return vulnerability.FilterWorkloadsBySBOMStatus(ctx, workloads, statuses, filter.HasVulnerabilityData)
}
