package deployment

import (
	"context"

	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

const (
	ApplicationOrderFieldDeploymentTime application.ApplicationOrderField = "DEPLOYMENT_TIME"
	JobOrderFieldDeploymentTime         job.JobOrderField                 = "DEPLOYMENT_TIME"
)

func init() {
	application.AllApplicationOrderField = append(application.AllApplicationOrderField, ApplicationOrderFieldDeploymentTime)
	job.AllJobOrderField = append(job.AllJobOrderField, JobOrderFieldDeploymentTime)

	sortByTimestamp := func(ctx context.Context, wl workload.Workload) int {
		info, err := InfoForWorkload(ctx, wl)
		if err != nil {
			return -1
		}

		if info.Timestamp == nil {
			return -1
		}
		return int(info.Timestamp.Unix())
	}

	application.SortFilter.RegisterConcurrentOrderBy(ApplicationOrderFieldDeploymentTime, func(ctx context.Context, a *application.Application) int {
		return sortByTimestamp(ctx, a)
	})

	job.SortFilter.RegisterConcurrentOrderBy(JobOrderFieldDeploymentTime, func(ctx context.Context, a *job.Job) int {
		return sortByTimestamp(ctx, a)
	})
}
