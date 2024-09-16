package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/job"
)

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

func (r *teamResolver) Jobs(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *job.JobOrder) (*pagination.Connection[*job.Job], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return job.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) Job(ctx context.Context, obj *team.TeamEnvironment, name string) (*job.Job, error) {
	return job.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *Resolver) Job() gengqlv1.JobResolver { return &jobResolver{r} }

type jobResolver struct{ *Resolver }
