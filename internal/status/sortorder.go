package status

import (
	"context"

	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

const (
	ApplicationOrderFieldStatus application.ApplicationOrderField = "STATUS"
	JobOrderFieldStatus         job.JobOrderField                 = "STATUS"
)

func init() {
	application.AllApplicationOrderField = append(application.AllApplicationOrderField, ApplicationOrderFieldStatus)
	job.AllJobOrderField = append(job.AllJobOrderField, JobOrderFieldStatus)

	application.SortFilter.RegisterConcurrentOrderBy(ApplicationOrderFieldStatus, func(ctx context.Context, a *application.Application) int {
		return int(ForWorkload(ctx, a).State)
	})

	job.SortFilter.RegisterConcurrentOrderBy(JobOrderFieldStatus, func(ctx context.Context, a *job.Job) int {
		return int(ForWorkload(ctx, a).State)
	})
}
