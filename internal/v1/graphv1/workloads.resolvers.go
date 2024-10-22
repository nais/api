package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *teamResolver) Workloads(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *workload.WorkloadOrder, filter *workload.TeamWorkloadsFilter) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	apps := application.ListAllForTeam(ctx, obj.Slug)
	jobs := job.ListAllForTeam(ctx, obj.Slug)

	workloads := make([]workload.Workload, 0, len(apps)+len(jobs))
	for _, app := range apps {
		workloads = append(workloads, app)
	}
	for _, job := range jobs {
		workloads = append(workloads, job)
	}

	filtered := workload.SortFilter.Filter(ctx, workloads, filter)
	if orderBy == nil {
		orderBy = &workload.WorkloadOrder{Field: workload.WorkloadOrderFieldName, Direction: modelv1.OrderDirectionAsc}
	}
	workload.SortFilter.Sort(ctx, filtered, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(filtered, page)
	return pagination.NewConnection(ret, page, int32(len(filtered))), nil
}

func (r *Resolver) ContainerImage() gengqlv1.ContainerImageResolver {
	return &containerImageResolver{r}
}

type containerImageResolver struct{ *Resolver }
