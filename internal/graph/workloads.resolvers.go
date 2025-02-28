package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *teamResolver) Workloads(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *workload.WorkloadOrder, filter *workload.TeamWorkloadsFilter) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	apps := application.ListAllForTeam(ctx, obj.Slug, nil, nil)
	jobs := job.ListAllForTeam(ctx, obj.Slug, nil, nil)

	workloads := make([]workload.Workload, 0, len(apps)+len(jobs))
	for _, app := range apps {
		workloads = append(workloads, app)
	}
	for _, j := range jobs {
		workloads = append(workloads, j)
	}

	filtered := workload.SortFilter.Filter(ctx, workloads, filter)
	if orderBy == nil {
		orderBy = &workload.WorkloadOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	workload.SortFilter.Sort(ctx, filtered, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(filtered, page)
	return pagination.NewConnection(ret, page, len(filtered)), nil
}

func (r *teamEnvironmentResolver) Workload(ctx context.Context, obj *team.TeamEnvironment, name string) (workload.Workload, error) {
	return tryWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *Resolver) ContainerImage() gengql.ContainerImageResolver { return &containerImageResolver{r} }

type containerImageResolver struct{ *Resolver }
