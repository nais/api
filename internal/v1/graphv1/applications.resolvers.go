package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload/application"
)

func (r *applicationResolver) Team(ctx context.Context, obj *application.Application) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *applicationResolver) Environment(ctx context.Context, obj *application.Application) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *teamResolver) Applications(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *application.ApplicationOrder) (*pagination.Connection[*application.Application], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return application.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) Application(ctx context.Context, obj *team.TeamEnvironment, name string) (*application.Application, error) {
	return application.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *Resolver) Application() gengqlv1.ApplicationResolver { return &applicationResolver{r} }

type applicationResolver struct{ *Resolver }
