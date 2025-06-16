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
		workload := ForWorkload(ctx, v)
		stateMatch := len(filter.States) == 0 || slices.Contains(filter.States, workload.State.String())
		hasErrors := len(workload.Errors) > 0

		switch {
		case filter.HasStatusErrors == nil:
			// Only filter by state (or no filter at all)
			return stateMatch
		case *filter.HasStatusErrors:
			// Filter by state AND must have errors
			return stateMatch && hasErrors
		default:
			// Filter by state AND must NOT have errors
			return stateMatch && !hasErrors
		}
	})
}
