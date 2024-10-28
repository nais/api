package status

import (
	"context"

	"github.com/nais/api/internal/v1/workload/application"
)

const ApplicationOrderFieldStatus application.ApplicationOrderField = "STATUS"

func init() {
	application.AllApplicationOrderField = append(application.AllApplicationOrderField, ApplicationOrderFieldStatus)

	application.SortFilter.RegisterConcurrentOrderBy(ApplicationOrderFieldStatus, func(ctx context.Context, a *application.Application) int {
		return int(ForWorkload(ctx, a).State)
	})
}
