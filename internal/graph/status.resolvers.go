package graph

import (
	"context"

	"github.com/nais/api/internal/status"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) Status(ctx context.Context, obj *application.Application) (*status.WorkloadStatus, error) {
	return status.ForWorkload(ctx, obj), nil
}

func (r *jobResolver) Status(ctx context.Context, obj *job.Job) (*status.WorkloadStatus, error) {
	return status.ForWorkload(ctx, obj), nil
}
