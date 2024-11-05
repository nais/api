package graphv1

import (
	"context"
	"math"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/role"
	"github.com/nais/api/internal/v1/status"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *deleteJobPayloadResolver) Team(ctx context.Context, obj *job.DeleteJobPayload) (*team.Team, error) {
	if obj.TeamSlug == nil {
		return nil, nil
	}
	return team.Get(ctx, *obj.TeamSlug)
}

func (r *jobResolver) Team(ctx context.Context, obj *job.Job) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *jobResolver) Environment(ctx context.Context, obj *job.Job) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *jobResolver) AuthIntegrations(ctx context.Context, obj *job.Job) ([]workload.JobAuthIntegrations, error) {
	ret := make([]workload.JobAuthIntegrations, 0)

	if v := workload.GetEntraIDAuthIntegrationForJob(obj.Spec.Azure); v != nil {
		ret = append(ret, v)
	}

	if v := workload.GetMaskinPortenAuthIntegration(obj.Spec.Maskinporten); v != nil {
		ret = append(ret, v)
	}

	return ret, nil
}

func (r *jobResolver) Runs(ctx context.Context, obj *job.Job, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*job.JobRun], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return job.Runs(ctx, obj.TeamSlug, obj.Name, page)
}

func (r *jobResolver) Manifest(ctx context.Context, obj *job.Job) (*job.JobManifest, error) {
	return job.Manifest(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
}

func (r *jobRunResolver) Duration(ctx context.Context, obj *job.JobRun) (int, error) {
	return int(math.Round(obj.Duration().Seconds())), nil
}

func (r *jobRunResolver) Instances(ctx context.Context, obj *job.JobRun, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*job.JobRunInstance], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return job.ListJobRunInstances(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, page)
}

func (r *mutationResolver) DeleteJob(ctx context.Context, input job.DeleteJobInput) (*job.DeleteJobPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationJobsDelete, input.TeamSlug); err != nil {
		return nil, err
	}
	return job.Delete(ctx, input.TeamSlug, input.EnvironmentName, input.Name)
}

func (r *mutationResolver) TriggerJob(ctx context.Context, input job.TriggerJobInput) (*job.TriggerJobPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationJobsUpdate, input.TeamSlug); err != nil {
		return nil, err
	}
	ret, err := job.Trigger(ctx, input.TeamSlug, input.EnvironmentName, input.Name, input.RunName)
	if err != nil {
		return nil, err
	}

	return &job.TriggerJobPayload{
		JobRun:          ret,
		JobName:         input.Name,
		TeamSlug:        input.TeamSlug,
		EnvironmentName: input.EnvironmentName,
	}, nil
}

func (r *teamResolver) Jobs(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *job.JobOrder, filter *job.TeamJobsFilter) (*pagination.Connection[*job.Job], error) {
	if filter == nil {
		filter = &job.TeamJobsFilter{}
	}
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	if orderBy == nil {
		orderBy = &job.JobOrder{
			Field:     job.JobOrderFieldName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}

	ret := job.ListAllForTeam(ctx, obj.Slug)
	ret = job.SortFilter.Filter(ctx, ret, filter)

	job.SortFilter.Sort(ctx, ret, orderBy.Field, orderBy.Direction)
	jobs := pagination.Slice(ret, page)
	return pagination.NewConnection(jobs, page, int32(len(ret))), nil
}

func (r *teamEnvironmentResolver) Job(ctx context.Context, obj *team.TeamEnvironment, name string) (*job.Job, error) {
	return job.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamInventoryCountsResolver) Jobs(ctx context.Context, obj *team.TeamInventoryCounts) (*job.TeamInventoryCountJobs, error) {
	apps := job.ListAllForTeam(ctx, obj.TeamSlug)
	notNais := 0

	for _, app := range apps {
		s := status.ForWorkload(ctx, app)
		if s.State == status.WorkloadStateNotNais {
			notNais++
		}
	}

	return &job.TeamInventoryCountJobs{
		Total:   len(apps),
		NotNais: notNais,
	}, nil
}

func (r *triggerJobPayloadResolver) Job(ctx context.Context, obj *job.TriggerJobPayload) (*job.Job, error) {
	return job.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.JobName)
}

func (r *Resolver) DeleteJobPayload() gengqlv1.DeleteJobPayloadResolver {
	return &deleteJobPayloadResolver{r}
}

func (r *Resolver) Job() gengqlv1.JobResolver { return &jobResolver{r} }

func (r *Resolver) JobRun() gengqlv1.JobRunResolver { return &jobRunResolver{r} }

func (r *Resolver) TriggerJobPayload() gengqlv1.TriggerJobPayloadResolver {
	return &triggerJobPayloadResolver{r}
}

type (
	deleteJobPayloadResolver  struct{ *Resolver }
	jobResolver               struct{ *Resolver }
	jobRunResolver            struct{ *Resolver }
	triggerJobPayloadResolver struct{ *Resolver }
)
