package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/graphv1/modelv1/donotuse"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload/application"
)

func (r *applicationResolver) Team(ctx context.Context, obj *application.Application) (*team.Team, error) {
	panic(fmt.Errorf("not implemented: Team - team"))
}

func (r *applicationResolver) Environment(ctx context.Context, obj *application.Application) (*donotuse.Environment, error) {
	panic(fmt.Errorf("not implemented: Environment - environment"))
}

func (r *Resolver) Application() gengqlv1.ApplicationResolver { return &applicationResolver{r} }

type applicationResolver struct{ *Resolver }
