package graph

import (
	"context"

	"github.com/nais/api/internal/alerts"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
)

func (r *teamEnvironmentResolver) Alerts(ctx context.Context, obj *team.TeamEnvironment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *alerts.AlertOrder, filter *alerts.TeamAlertsFilter) (*pagination.Connection[alerts.Alert], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	prometheusAlerts, err := alerts.ListPrometheusAlerts(ctx, obj.EnvironmentName, obj.TeamSlug)
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

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
/*
	func (r *prometheusAlertResolver) State(ctx context.Context, obj *alerts.PrometheusAlert) (alerts.AlertState, error) {
	panic(fmt.Errorf("not implemented: State - state"))
}
func (r *Resolver) PrometheusAlert() gengql.PrometheusAlertResolver {
	return &prometheusAlertResolver{r}
}
type prometheusAlertResolver struct{ *Resolver }
*/
