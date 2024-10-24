package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/status"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *applicationResolver) Status(ctx context.Context, obj *application.Application) (*status.WorkloadStatus, error) {
	return status.ForWorkload(ctx, obj), nil
}

func (r *jobResolver) Status(ctx context.Context, obj *job.Job) (*status.WorkloadStatus, error) {
	return status.ForWorkload(ctx, obj), nil
}
