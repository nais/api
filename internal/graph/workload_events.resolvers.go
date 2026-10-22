package graph

import (
	"context"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/event"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) Events(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *event.WorkloadEventFilter) (*pagination.Connection[event.WorkloadEvent], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return event.ListForApplication(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, page, filter)
}

func (r *jobResolver) Events(ctx context.Context, obj *job.Job, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *event.WorkloadEventFilter) (*pagination.Connection[event.WorkloadEvent], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return event.ListForJob(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, page, filter)
}
