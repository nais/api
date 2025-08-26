package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/alerts"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
)

func (r *prometheusAlertResolver) Team(ctx context.Context, obj *alerts.PrometheusAlert) (*team.Team, error) {
	team, err := team.Get(ctx, obj.TeamSlug)
	if err != nil {
		fmt.Println("Error getting team: ", obj.TeamSlug, err)
	}

	return team, err
}

func (r *prometheusAlertResolver) TeamEnvironment(ctx context.Context, obj *alerts.PrometheusAlert) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *teamResolver) Alerts(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *alerts.AlertOrder, filter *alerts.TeamAlertsFilter) (*pagination.Connection[alerts.Alert], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	prometheusAlerts, err := alerts.ListPrometheusRulesForTeam(ctx, obj.Slug)
	if err != nil {
		return nil, err
	}

	a := make([]alerts.Alert, 0, len(prometheusAlerts))
	for _, alert := range prometheusAlerts {
		a = append(a, alert)
	}

	filtered := alerts.SortFilter.Filter(ctx, a, filter)
	if orderBy == nil {
		orderBy = &alerts.AlertOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	alerts.SortFilter.Sort(ctx, filtered, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(filtered, page)
	return pagination.NewConnection(ret, page, len(filtered)), nil
}

func (r *teamEnvironmentResolver) Alerts(ctx context.Context, obj *team.TeamEnvironment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *alerts.AlertOrder, filter *alerts.TeamAlertsFilter) (*pagination.Connection[alerts.Alert], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	prometheusAlerts, err := alerts.ListPrometheusRulesForTeamInEnvironment(ctx, obj.EnvironmentName, obj.TeamSlug)
	if err != nil {
		return nil, err
	}

	a := make([]alerts.Alert, 0, len(prometheusAlerts))
	for _, alert := range prometheusAlerts {
		a = append(a, alert)
	}

	filtered := alerts.SortFilter.Filter(ctx, a, filter)
	if orderBy == nil {
		orderBy = &alerts.AlertOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	alerts.SortFilter.Sort(ctx, filtered, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(filtered, page)
	return pagination.NewConnection(ret, page, len(filtered)), nil
}

func (r *Resolver) PrometheusAlert() gengql.PrometheusAlertResolver {
	return &prometheusAlertResolver{r}
}

type prometheusAlertResolver struct{ *Resolver }
