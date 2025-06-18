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
		errorMatch := len(filter.WorkloadStatusErrorTypes) == 0

		for _, err := range workload.Errors {
			if slices.Contains(filter.WorkloadStatusErrorTypes, err.GetName()) {
				errorMatch = true
				break
			}
		}
		return stateMatch && errorMatch
	})
}
