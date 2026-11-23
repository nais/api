package graph

import (
	"context"
	"slices"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/vulnerability"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *containerImageResolver) ActivityLog(ctx context.Context, obj *workload.ContainerImage, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*activitylog.ActivityLogEntryConnection, error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return activitylog.ListForResource(ctx, "VULNERABILITY", obj.Name, page, filter)
}

func (r *environmentResolver) Workloads(ctx context.Context, obj *environment.Environment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *workload.EnvironmentWorkloadOrder) (*workload.WorkloadConnection, error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	apps := application.ListAllInEnvironment(ctx, obj.Name)
	jobs := job.ListAllInEnvironment(ctx, obj.Name)

	workloads := make([]workload.Workload, 0, len(apps)+len(jobs))
	for _, app := range apps {
		workloads = append(workloads, app)
	}
	for _, j := range jobs {
		workloads = append(workloads, j)
	}

	if orderBy == nil {
		orderBy = &workload.EnvironmentWorkloadOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	workload.SortFilterEnvironment.Sort(ctx, workloads, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(workloads, page)
	return newWorkloadConnection(v0s.Scope{Environment: obj.Name}, workloads, ret, page, nil), nil
}

func (r *teamResolver) Workloads(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *workload.WorkloadOrder, filter *workload.TeamWorkloadsFilter) (*workload.WorkloadConnection, error) {
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
	filtered, err = filterWorkloadsBySBOMStatus(ctx, filtered, filter)
	if err != nil {
		return nil, err
	}
	scope := v0s.Scope{Team: obj.Slug}
	var coverage *workload.VulnerabilityCoverageCounts
	if filter != nil && len(filter.VulnerabilityCoverageStatuses) > 0 {
		statuses, err := v0s.WorkloadCoverageStatuses(ctx, scope, filtered)
		if err != nil {
			return nil, err
		}
		selected := make([]v0s.VulnerabilityCoverageStatus, 0, len(filtered))
		matches := filtered[:0]
		for i, status := range statuses {
			if slices.Contains(filter.VulnerabilityCoverageStatuses, string(status)) {
				matches = append(matches, filtered[i])
				selected = append(selected, status)
			}
		}
		filtered = matches
		coverage = countWorkloadCoverage(selected)
	}
	if orderBy == nil {
		orderBy = &workload.WorkloadOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	workload.SortFilter.Sort(ctx, filtered, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(filtered, page)
	return newWorkloadConnection(scope, filtered, ret, page, coverage), nil
}

func (r *teamEnvironmentResolver) Workload(ctx context.Context, obj *team.TeamEnvironment, name string) (workload.Workload, error) {
	return tryWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamWorkloadsFilterResolver) SbomStatus(ctx context.Context, obj *workload.TeamWorkloadsFilter, data *vulnerability.SBOMStatus) error {
	if data != nil {
		status := data.String()
		obj.SbomStatus = &status
	}

	return nil
}

func (r *teamWorkloadsFilterResolver) VulnerabilityCoverageStatuses(ctx context.Context, obj *workload.TeamWorkloadsFilter, data []v0s.VulnerabilityCoverageStatus) error {
	for _, status := range data {
		obj.VulnerabilityCoverageStatuses = append(obj.VulnerabilityCoverageStatuses, string(status))
	}
	return nil
}

func (r *Resolver) ContainerImage() gengql.ContainerImageResolver { return &containerImageResolver{r} }

func (r *Resolver) TeamWorkloadsFilter() gengql.TeamWorkloadsFilterResolver {
	return &teamWorkloadsFilterResolver{r}
}

type (
	containerImageResolver      struct{ *Resolver }
	teamWorkloadsFilterResolver struct{ *Resolver }
)
