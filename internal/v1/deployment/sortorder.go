package deployment

import (
	"context"

	"github.com/nais/api/internal/v1/workload/application"
)

const ApplicationOrderFieldDeploymentTime application.ApplicationOrderField = "DEPLOYMENT_TIME"

func init() {
	application.AllApplicationOrderField = append(application.AllApplicationOrderField, ApplicationOrderFieldDeploymentTime)

	application.SortFilter.RegisterConcurrentOrderBy(ApplicationOrderFieldDeploymentTime, func(ctx context.Context, a *application.Application) int {
		info, err := InfoForWorkload(ctx, a)
		if err != nil {
			return -1
		}

		if info.Timestamp == nil {
			return -1
		}
		return int(info.Timestamp.Unix())
	})
}
