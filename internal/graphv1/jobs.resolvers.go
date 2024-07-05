package graphv1

import (
	"context"
	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload/job"
)

func (r *jobResolver) Team(ctx context.Context, obj *job.Job) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *jobResolver) Environment(ctx context.Context, obj *job.Job) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *Resolver) Job() gengqlv1.JobResolver { return &jobResolver{r} }

type jobResolver struct{ *Resolver }
