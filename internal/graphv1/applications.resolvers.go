package graphv1

import (
	"context"

	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload/application"
)

func (r *applicationResolver) Team(ctx context.Context, obj *application.Application) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *applicationResolver) Environment(ctx context.Context, obj *application.Application) (*team.TeamEnvironment, error) {
	return &team.TeamEnvironment{
		Name: obj.EnvironmentName,
	}, nil
}

func (r *Resolver) Application() gengqlv1.ApplicationResolver { return &applicationResolver{r} }

type applicationResolver struct{ *Resolver }
