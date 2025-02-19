package graph

import (
	"context"

	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/nais/api/internal/workload/logging"
)

func (r *applicationResolver) LogDestinations(ctx context.Context, obj *application.Application) ([]logging.LogDestination, error) {
	return logging.FromWorkload(ctx, obj), nil
}

func (r *jobResolver) LogDestinations(ctx context.Context, obj *job.Job) ([]logging.LogDestination, error) {
	return logging.FromWorkload(ctx, obj), nil
}
