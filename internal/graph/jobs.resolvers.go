package graph

import (
	"context"
	"math"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/status"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/job"
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
	return r.TeamEnvironment(ctx, obj)
}

func (r *jobResolver) TeamEnvironment(ctx context.Context, obj *job.Job) (*team.TeamEnvironment, error) {
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

	return job.Runs(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, page)
}

func (r *jobResolver) Manifest(ctx context.Context, obj *job.Job) (*job.JobManifest, error) {
	return job.Manifest(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
}

func (r *jobResolver) ActivityLog(ctx context.Context, obj *job.Job, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[activitylog.ActivityLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return activitylog.ListForResourceTeamAndEnvironment(ctx, "JOB", obj.TeamSlug, obj.Name, obj.EnvironmentName, page)
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
	if err := authz.CanDeleteJobs(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	return job.Delete(ctx, input.TeamSlug, input.EnvironmentName, input.Name)
}

func (r *mutationResolver) TriggerJob(ctx context.Context, input job.TriggerJobInput) (*job.TriggerJobPayload, error) {
	if err := authz.CanUpdateJobs(ctx, input.TeamSlug); err != nil {
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
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret := job.ListAllForTeam(ctx, obj.Slug, orderBy, filter)
	jobs := pagination.Slice(ret, page)
	return pagination.NewConnection(jobs, page, len(ret)), nil
}

func (r *teamEnvironmentResolver) Job(ctx context.Context, obj *team.TeamEnvironment, name string) (*job.Job, error) {
	return job.Get(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountJobsResolver) NotNais(ctx context.Context, obj *job.TeamInventoryCountJobs) (int, error) {
	jobs := job.ListAllForTeam(ctx, obj.TeamSlug, nil, nil)
	notNais := 0

	for _, j := range jobs {
		s := status.ForWorkload(ctx, j)
		if s.State == status.WorkloadStateNotNais {
			notNais++
		}
	}
	return notNais, nil
}

func (r *teamInventoryCountsResolver) Jobs(ctx context.Context, obj *team.TeamInventoryCounts) (*job.TeamInventoryCountJobs, error) {
	jobs := job.ListAllForTeam(ctx, obj.TeamSlug, nil, nil)

	return &job.TeamInventoryCountJobs{
		Total:    len(jobs),
		TeamSlug: obj.TeamSlug,
	}, nil
}

func (r *triggerJobPayloadResolver) Job(ctx context.Context, obj *job.TriggerJobPayload) (*job.Job, error) {
	return job.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.JobName)
}

func (r *Resolver) DeleteJobPayload() gengql.DeleteJobPayloadResolver {
	return &deleteJobPayloadResolver{r}
}

func (r *Resolver) Job() gengql.JobResolver { return &jobResolver{r} }

func (r *Resolver) JobRun() gengql.JobRunResolver { return &jobRunResolver{r} }

func (r *Resolver) TeamInventoryCountJobs() gengql.TeamInventoryCountJobsResolver {
	return &teamInventoryCountJobsResolver{r}
}

func (r *Resolver) TriggerJobPayload() gengql.TriggerJobPayloadResolver {
	return &triggerJobPayloadResolver{r}
}

type (
	deleteJobPayloadResolver       struct{ *Resolver }
	jobResolver                    struct{ *Resolver }
	jobRunResolver                 struct{ *Resolver }
	teamInventoryCountJobsResolver struct{ *Resolver }
	triggerJobPayloadResolver      struct{ *Resolver }
)
