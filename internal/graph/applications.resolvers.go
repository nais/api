package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/status"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
)

func (r *applicationResolver) Team(ctx context.Context, obj *application.Application) (*team.Team, error) {
	team, err := team.Get(ctx, obj.TeamSlug)
	if err != nil {
		fmt.Println("Error getting team: ", obj.TeamSlug, err)
	}

	return team, err
}

func (r *applicationResolver) Environment(ctx context.Context, obj *application.Application) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *applicationResolver) TeamEnvironment(ctx context.Context, obj *application.Application) (*team.TeamEnvironment, error) {
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

func (r *applicationResolver) ActivityLog(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*pagination.Connection[activitylog.ActivityLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return activitylog.ListForResourceTeamAndEnvironment(ctx, "APP", obj.TeamSlug, obj.Name, obj.EnvironmentName, page, filter)
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

func (r *ingressMetricsResolver) RequestsPerSecond(ctx context.Context, obj *application.IngressMetrics) (float64, error) {
	return application.RequestsPerSecondForIngress(ctx), nil
}

func (r *ingressMetricsResolver) ErrorsPerSecond(ctx context.Context, obj *application.IngressMetrics) (float64, error) {
	return application.ErrorsPerSecondForIngress(ctx), nil
}

func (r *mutationResolver) DeleteApplication(ctx context.Context, input application.DeleteApplicationInput) (*application.DeleteApplicationPayload, error) {
	if err := authz.CanDeleteApplications(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	return application.Delete(ctx, input.TeamSlug, input.EnvironmentName, input.Name)
}

func (r *mutationResolver) RestartApplication(ctx context.Context, input application.RestartApplicationInput) (*application.RestartApplicationPayload, error) {
	if err := authz.CanUpdateApplications(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := application.Restart(ctx, input.TeamSlug, input.EnvironmentName, input.Name); err != nil {
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
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret := application.ListAllForTeam(ctx, obj.Slug, orderBy, filter)
	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, len(ret)), nil
}

func (r *teamEnvironmentResolver) Application(ctx context.Context, obj *team.TeamEnvironment, name string) (*application.Application, error) {
	return application.Get(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountApplicationsResolver) NotNais(ctx context.Context, obj *application.TeamInventoryCountApplications) (int, error) {
	apps := application.ListAllForTeam(ctx, obj.TeamSlug, nil, nil)
	notNais := 0

	for _, app := range apps {
		s := status.ForWorkload(ctx, app)
		if s.State == status.WorkloadStateNotNais {
			notNais++
		}
	}
	return notNais, nil
}

func (r *teamInventoryCountsResolver) Applications(ctx context.Context, obj *team.TeamInventoryCounts) (*application.TeamInventoryCountApplications, error) {
	apps := application.ListAllForTeam(ctx, obj.TeamSlug, nil, nil)

	return &application.TeamInventoryCountApplications{
		Total:    len(apps),
		TeamSlug: obj.TeamSlug,
	}, nil
}

func (r *Resolver) Application() gengql.ApplicationResolver { return &applicationResolver{r} }

func (r *Resolver) ApplicationInstance() gengql.ApplicationInstanceResolver {
	return &applicationInstanceResolver{r}
}

func (r *Resolver) DeleteApplicationPayload() gengql.DeleteApplicationPayloadResolver {
	return &deleteApplicationPayloadResolver{r}
}

func (r *Resolver) Ingress() gengql.IngressResolver { return &ingressResolver{r} }

func (r *Resolver) IngressMetrics() gengql.IngressMetricsResolver { return &ingressMetricsResolver{r} }

func (r *Resolver) RestartApplicationPayload() gengql.RestartApplicationPayloadResolver {
	return &restartApplicationPayloadResolver{r}
}

func (r *Resolver) TeamInventoryCountApplications() gengql.TeamInventoryCountApplicationsResolver {
	return &teamInventoryCountApplicationsResolver{r}
}

type (
	applicationResolver                    struct{ *Resolver }
	applicationInstanceResolver            struct{ *Resolver }
	deleteApplicationPayloadResolver       struct{ *Resolver }
	ingressResolver                        struct{ *Resolver }
	ingressMetricsResolver                 struct{ *Resolver }
	restartApplicationPayloadResolver      struct{ *Resolver }
	teamInventoryCountApplicationsResolver struct{ *Resolver }
)
