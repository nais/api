package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/status"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
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

func (r *applicationResolver) Instances(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*application.ApplicationInstance], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return application.ListInstances(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, page)
}

func (r *deleteApplicationPayloadResolver) Team(ctx context.Context, obj *application.DeleteApplicationPayload) (*team.Team, error) {
	if obj.TeamSlug == nil {
		return nil, nil
	}

	return team.Get(ctx, *obj.TeamSlug)
}

func (r *ingressResolver) Type(ctx context.Context, obj *application.Ingress) (application.IngressType, error) {
	return application.GetIngressType(ctx, obj), nil
}

func (r *mutationResolver) DeleteApplication(ctx context.Context, input application.DeleteApplicationInput) (*application.DeleteApplicationPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationApplicationsDelete, input.TeamSlug); err != nil {
		return nil, err
	}
	return application.Delete(ctx, input.TeamSlug, input.EnvironmentName, input.Name)
}

func (r *mutationResolver) RestartApplication(ctx context.Context, input application.RestartApplicationInput) (*application.RestartApplicationPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationApplicationsUpdate, input.TeamSlug); err != nil {
		return nil, err
	}
	err := application.Restart(ctx, input.TeamSlug, input.EnvironmentName, input.Name)
	if err != nil {
		return nil, err
	}

	return &application.RestartApplicationPayload{
		TeamSlug:        input.TeamSlug,
		EnvironmentName: input.EnvironmentName,
		ApplicationName: input.Name,
	}, nil
}

func (r *restartApplicationPayloadResolver) Application(ctx context.Context, obj *application.RestartApplicationPayload) (*application.Application, error) {
	return application.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.ApplicationName)
}

func (r *teamResolver) Applications(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *application.ApplicationOrder, filter *application.TeamApplicationsFilter) (*pagination.Connection[*application.Application], error) {
	if filter == nil {
		filter = &application.TeamApplicationsFilter{}
	}

	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	if orderBy == nil {
		orderBy = &application.ApplicationOrder{
			Field:     application.ApplicationOrderFieldName,
			Direction: model.OrderDirectionAsc,
		}
	}

	ret := application.ListAllForTeam(ctx, obj.Slug)
	ret = application.SortFilter.Filter(ctx, ret, filter)

	application.SortFilter.Sort(ctx, ret, orderBy.Field, orderBy.Direction)
	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, int32(len(ret))), nil
}

func (r *teamEnvironmentResolver) Application(ctx context.Context, obj *team.TeamEnvironment, name string) (*application.Application, error) {
	return application.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamInventoryCountsResolver) Applications(ctx context.Context, obj *team.TeamInventoryCounts) (*application.TeamInventoryCountApplications, error) {
	apps := application.ListAllForTeam(ctx, obj.TeamSlug)
	notNais := 0

	for _, app := range apps {
		s := status.ForWorkload(ctx, app)
		if s.State == status.WorkloadStateNotNais {
			notNais++
		}
	}

	return &application.TeamInventoryCountApplications{
		Total:   len(apps),
		NotNais: notNais,
	}, nil
}

func (r *Resolver) Application() gengql.ApplicationResolver { return &applicationResolver{r} }

func (r *Resolver) DeleteApplicationPayload() gengql.DeleteApplicationPayloadResolver {
	return &deleteApplicationPayloadResolver{r}
}

func (r *Resolver) Ingress() gengql.IngressResolver { return &ingressResolver{r} }

func (r *Resolver) RestartApplicationPayload() gengql.RestartApplicationPayloadResolver {
	return &restartApplicationPayloadResolver{r}
}

type (
	applicationResolver               struct{ *Resolver }
	deleteApplicationPayloadResolver  struct{ *Resolver }
	ingressResolver                   struct{ *Resolver }
	restartApplicationPayloadResolver struct{ *Resolver }
)