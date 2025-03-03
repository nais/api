package status

import (
	"context"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

const (
	ApplicationOrderFieldStatus application.ApplicationOrderField = "STATUS"
	JobOrderFieldStatus         job.JobOrderField                 = "STATUS"
)

func init() {
	application.SortFilter.RegisterConcurrentOrderBy(ApplicationOrderFieldStatus, func(ctx context.Context, a *application.Application) int {
		return int(ForWorkload(ctx, a).State)
	})

	job.SortFilter.RegisterConcurrentOrderBy(JobOrderFieldStatus, func(ctx context.Context, a *job.Job) int {
		return int(ForWorkload(ctx, a).State)
	})

	workload.SortFilter.RegisterConcurrentOrderBy(workload.WorkloadOrderFieldStatus, func(ctx context.Context, a workload.Workload) int {
		return int(ForWorkload(ctx, a).State)
	})
}
