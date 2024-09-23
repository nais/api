package graphv1

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *teamResolver) Workloads(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *workload.WorkloadOrder) (*pagination.Connection[workload.Workload], error) {
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

	// Sort by name first, then the other defined by the order
	nameOrder := func(a workload.Workload, b workload.Workload) int {
		return strings.Compare(a.GetName(), b.GetName())
	}
	slices.SortStableFunc(workloads, nameOrder)

	if orderBy == nil {
		orderBy = &workload.WorkloadOrder{
			Field: workload.WorkloadOrderFieldName,
		}
	}

	var cmp func(a workload.Workload, b workload.Workload) int

	switch orderBy.Field {
	case workload.WorkloadOrderFieldName:
		// already sorted by name
	case workload.WorkloadOrderFieldDeploymentTime:
		// not supported yet
	case workload.WorkloadOrderFieldEnvironment:
		cmp = func(a, b workload.Workload) int {
			return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
		}
	}

	if cmp != nil {
		slices.SortStableFunc(workloads, func(a, b workload.Workload) int {
			if orderBy.Direction == modelv1.OrderDirectionDesc {
				return cmp(b, a) * -1
			}
			return cmp(a, b)
		})
	}

	ret := pagination.Slice(workloads, page)
	return pagination.NewConnection(ret, page, int32(len(workloads))), nil
}

func (r *Resolver) ContainerImage() gengqlv1.ContainerImageResolver {
	return &containerImageResolver{r}
}

type containerImageResolver struct{ *Resolver }
