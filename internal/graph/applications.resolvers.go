package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/issue"
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

func (r *applicationResolver) ActivityLog(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*activitylog.ActivityLogEntryConnection, error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return activitylog.ListForResourceTeamAndEnvironment(ctx, "APP", obj.TeamSlug, obj.Name, obj.EnvironmentName, page, filter)
}

func (r *applicationResolver) State(ctx context.Context, obj *application.Application) (application.ApplicationState, error) {
	return application.GetState(ctx, obj)
}

func (r *applicationResolver) Issues(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *issue.IssueOrder, filter *issue.ResourceIssueFilter) (*pagination.Connection[issue.Issue], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	t := issue.ResourceTypeApplication
	f := &issue.IssueFilter{
		ResourceName: &obj.Name,
		ResourceType: &t,
		Environments: []string{obj.EnvironmentName},
	}
	if filter != nil {
		f.Severity = filter.Severity
		f.IssueType = filter.IssueType
	}

	return issue.ListIssues(ctx, obj.TeamSlug, page, orderBy, f)
}

func (r *applicationConnectionResolver) Facets(ctx context.Context, obj *application.ApplicationConnection) (*application.ApplicationFacets, error) {
	return application.ComputeFacets(ctx, obj.GetAllApps(), obj.GetFilter())
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

func (r *ingressResolver) Metrics(ctx context.Context, obj *application.Ingress) (*application.IngressMetrics, error) {
	return &application.IngressMetrics{
		Ingress: obj,
	}, nil
}

func (r *ingressMetricsResolver) RequestsPerSecond(ctx context.Context, obj *application.IngressMetrics) (float64, error) {
	return application.RequestsPerSecondForIngress(ctx, obj)
}

func (r *ingressMetricsResolver) ErrorsPerSecond(ctx context.Context, obj *application.IngressMetrics) (float64, error) {
	return application.ErrorsPerSecondForIngress(ctx, obj)
}

func (r *ingressMetricsResolver) Series(ctx context.Context, obj *application.IngressMetrics, input application.IngressMetricsInput) ([]*application.IngressMetricSample, error) {
	return application.SeriesForIngress(ctx, obj, input)
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

func (r *teamResolver) Applications(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *application.ApplicationOrder, filter *application.TeamApplicationsFilter) (*application.ApplicationConnection, error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	// Fetch all apps for the team (unfiltered) for facet computation.
	// Pass nil orderBy to avoid expensive sorting (e.g. STATE) on items that may be filtered out.
	unfilteredApps := application.ListAllForTeam(ctx, obj.Slug, nil, nil)

	// Apply filter for the actual result set.
	filteredApps := unfilteredApps
	if filter != nil {
		filteredApps = application.SortFilter.Filter(ctx, unfilteredApps, filter)
	}

	// Sort only the filtered result set.
	if orderBy == nil {
		orderBy = &application.ApplicationOrder{Field: "NAME", Direction: model.OrderDirectionAsc}
	}
	application.SortFilter.Sort(ctx, filteredApps, orderBy.Field, orderBy.Direction)

	apps := pagination.Slice(filteredApps, page)
	conn := pagination.NewConnection(apps, page, len(filteredApps))

	return application.NewApplicationConnection(conn, unfilteredApps, filter), nil
}

func (r *teamEnvironmentResolver) Application(ctx context.Context, obj *team.TeamEnvironment, name string) (*application.Application, error) {
	return application.Get(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountsResolver) Applications(ctx context.Context, obj *team.TeamInventoryCounts) (*application.TeamInventoryCountApplications, error) {
	apps := application.ListAllForTeam(ctx, obj.TeamSlug, nil, nil)
	running, notRunning, unknown := application.StateCounts(ctx, apps)

	return &application.TeamInventoryCountApplications{
		Total:      len(apps),
		Running:    running,
		NotRunning: notRunning,
		Unknown:    unknown,
		TeamSlug:   obj.TeamSlug,
	}, nil
}

func (r *Resolver) Application() gengql.ApplicationResolver { return &applicationResolver{r} }

func (r *Resolver) ApplicationConnection() gengql.ApplicationConnectionResolver {
	return &applicationConnectionResolver{r}
}

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

type (
	applicationResolver               struct{ *Resolver }
	applicationConnectionResolver     struct{ *Resolver }
	applicationInstanceResolver       struct{ *Resolver }
	deleteApplicationPayloadResolver  struct{ *Resolver }
	ingressResolver                   struct{ *Resolver }
	ingressMetricsResolver            struct{ *Resolver }
	restartApplicationPayloadResolver struct{ *Resolver }
)
