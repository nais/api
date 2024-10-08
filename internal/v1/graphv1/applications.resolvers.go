package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
)

func (r *applicationResolver) Team(ctx context.Context, obj *application.Application) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *applicationResolver) Environment(ctx context.Context, obj *application.Application) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *applicationResolver) AuthIntegrations(ctx context.Context, obj *application.Application) ([]workload.ApplicationAuthIntegrations, error) {
	ret := make([]workload.ApplicationAuthIntegrations, 0)

	if v := workload.GetEntraIDAuthIntegrationForApplication(obj.Spec.Azure); v != nil {
		ret = append(ret, v)
	}

	if v := workload.GetMaskinPortenAuthIntegration(obj.Spec.Maskinporten); v != nil {
		ret = append(ret, v)
	}

	if v := workload.GetTokenXAuthIntegration(obj.Spec.TokenX); v != nil {
		ret = append(ret, v)
	}

	if v := workload.GetIDPortenAuthIntegration(obj.Spec.IDPorten); v != nil {
		ret = append(ret, v)
	}

	return ret, nil
}

func (r *applicationResolver) Manifest(ctx context.Context, obj *application.Application) (*application.ApplicationManifest, error) {
	return application.Manifest(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
}

func (r *deleteApplicationPayloadResolver) Team(ctx context.Context, obj *application.DeleteApplicationPayload) (*team.Team, error) {
	if obj.TeamSlug == nil {
		return nil, nil
	}

	return team.Get(ctx, *obj.TeamSlug)
}

func (r *mutationResolver) DeleteApplication(ctx context.Context, input application.DeleteApplicationInput) (*application.DeleteApplicationPayload, error) {
	return application.Delete(ctx, input.TeamSlug, input.EnvironmentName, input.Name)
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

func (r *Resolver) DeleteApplicationPayload() gengqlv1.DeleteApplicationPayloadResolver {
	return &deleteApplicationPayloadResolver{r}
}

type (
	applicationResolver              struct{ *Resolver }
	deleteApplicationPayloadResolver struct{ *Resolver }
)
