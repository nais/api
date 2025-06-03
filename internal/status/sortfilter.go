package status

import (
	"context"
	"slices"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func init() {
	application.SortFilter.RegisterConcurrentSort("STATUS", func(ctx context.Context, a *application.Application) int {
		return int(ForWorkload(ctx, a).State)
	}, "NAME", "ENVIRONMENT")

	job.SortFilter.RegisterConcurrentSort("STATUS", func(ctx context.Context, a *job.Job) int {
		return int(ForWorkload(ctx, a).State)
	}, "NAME", "ENVIRONMENT")

	workload.SortFilter.RegisterConcurrentSort("STATUS", func(ctx context.Context, a workload.Workload) int {
		return int(ForWorkload(ctx, a).State)
	}, "NAME", "ENVIRONMENT")

	workload.SortFilter.RegisterFilter(func(ctx context.Context, v workload.Workload, filter *workload.TeamWorkloadsFilter) bool {
		if len(filter.WithStates) == 0 {
			return true
		}

		workloadState := ForWorkload(ctx, v).State
		return slices.Contains(filter.WithStates, workloadState.String())
	})
}
