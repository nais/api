package graph

import (
	"context"

	"github.com/nais/api/internal/alerts"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/model/donotuse"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
)

func (r *teamEnvironmentResolver) Alerts(ctx context.Context, obj *team.TeamEnvironment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *alerts.AlertOrder, filter *donotuse.AlertsFilter) (*pagination.Connection[alerts.Alert], error) {
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
