package status

import (
	"context"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func init() {
	application.SortFilter.RegisterConcurrentSort("STATUS", func(ctx context.Context, a *application.Application) int {
		return int(ForWorkload(ctx, a).State)
	})

	job.SortFilter.RegisterConcurrentSort("STATUS", func(ctx context.Context, a *job.Job) int {
		return int(ForWorkload(ctx, a).State)
	})

	workload.SortFilter.RegisterConcurrentSort("STATUS", func(ctx context.Context, a workload.Workload) int {
		return int(ForWorkload(ctx, a).State)
	})
}
