package workload

import (
	"context"

	"github.com/nais/api/internal/graph/pagination"
)

type WorkloadConnection struct {
	pagination.Connection[Workload]
	Facets *WorkloadFacets
}

type WorkloadFacets struct {
	LoadCoverage func(context.Context) (*VulnerabilityCoverageCounts, error)
}

func (f *WorkloadFacets) VulnerabilityCoverage(ctx context.Context) (*VulnerabilityCoverageCounts, error) {
	return f.LoadCoverage(ctx)
}

type VulnerabilityCoverageCounts struct {
	TotalWorkloads, Scanned, Pending, NoSbom, Failed, UnresolvedImage int
}
