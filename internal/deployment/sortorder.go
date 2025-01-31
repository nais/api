package deployment

import (
	"context"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

const (
	ApplicationOrderFieldDeploymentTime application.ApplicationOrderField = "DEPLOYMENT_TIME"
	JobOrderFieldDeploymentTime         job.JobOrderField                 = "DEPLOYMENT_TIME"
)

func init() {
	application.AllApplicationOrderField = append(application.AllApplicationOrderField, ApplicationOrderFieldDeploymentTime)
	job.AllJobOrderField = append(job.AllJobOrderField, JobOrderFieldDeploymentTime)

	sortByTimestamp := func(ctx context.Context, wl workload.Workload) int {
		ts, err := latestDeploymentTimestampForWorkload(ctx, wl)
		if err != nil {
			return -1
		}

		return int(ts.Unix())
	}

	application.SortFilter.RegisterConcurrentOrderBy(ApplicationOrderFieldDeploymentTime, func(ctx context.Context, a *application.Application) int {
		return sortByTimestamp(ctx, a)
	})

	job.SortFilter.RegisterConcurrentOrderBy(JobOrderFieldDeploymentTime, func(ctx context.Context, a *job.Job) int {
		return sortByTimestamp(ctx, a)
	})
}
