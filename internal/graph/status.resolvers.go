package graph

import (
	"context"

	"github.com/nais/api/internal/status"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) Status(ctx context.Context, obj *application.Application) (*status.WorkloadStatus, error) {
	return status.ForWorkload(ctx, obj), nil
}

func (r *jobResolver) Status(ctx context.Context, obj *job.Job) (*status.WorkloadStatus, error) {
	return status.ForWorkload(ctx, obj), nil
}

func (r *teamWorkloadsFilterResolver) States(ctx context.Context, obj *workload.TeamWorkloadsFilter, data []status.WorkloadState) error {
	if len(data) == 0 {
		return nil
	}

	obj.States = make([]string, len(data))
	for i, state := range data {
		obj.States[i] = state.String()
	}
	return nil
}

func (r *teamWorkloadsFilterResolver) StatusErrorTypes(ctx context.Context, obj *workload.TeamWorkloadsFilter, data []status.WorkloadStatusErrorType) error {
	if len(data) == 0 {
		return nil
	}

	obj.WorkloadStatusErrorTypes = make([]string, len(data))
	for i, errorType := range data {
		obj.WorkloadStatusErrorTypes[i] = errorType.String()
	}
	return nil
}
