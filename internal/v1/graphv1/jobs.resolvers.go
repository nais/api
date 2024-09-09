package graphv1

import (
	"context"
	"fmt"

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

func (r *teamResolver) Jobs(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *job.JobOrder) (*pagination.Connection[*job.Job], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return job.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *Resolver) Job() gengqlv1.JobResolver { return &jobResolver{r} }

type jobResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//   - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//     it when you're done.
//   - You have helper methods in this file. Move them out to keep these resolver files clean.
func (r *jobResolver) Image(ctx context.Context, obj *job.Job) (*workload.ContainerImage, error) {
	panic(fmt.Errorf("not implemented: Image - image"))
}
