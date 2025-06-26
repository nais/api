package deployment

import (
	"context"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func init() {
	sortByTimestamp := func(ctx context.Context, wl workload.Workload) int {
		ts, err := latestDeploymentTimestampForWorkload(ctx, wl)
		if err != nil {
			return -1
		}

		return int(ts.Unix())
	}

	application.SortFilter.RegisterConcurrentSort("DEPLOYMENT_TIME", func(ctx context.Context, a *application.Application) int {
		return sortByTimestamp(ctx, a)
	}, "NAME", "ENVIRONMENT")

	job.SortFilter.RegisterConcurrentSort("DEPLOYMENT_TIME", func(ctx context.Context, a *job.Job) int {
		return sortByTimestamp(ctx, a)
	}, "NAME", "ENVIRONMENT")

	workload.SortFilter.RegisterConcurrentSort("DEPLOYMENT_TIME", func(ctx context.Context, a workload.Workload) int {
		return sortByTimestamp(ctx, a)
	}, "NAME", "ENVIRONMENT", "_KIND")

	workload.SortFilterEnvironment.RegisterConcurrentSort("DEPLOYMENT_TIME", func(ctx context.Context, a workload.Workload) int {
		return sortByTimestamp(ctx, a)
	}, "NAME", "TEAM_SLUG")
}
