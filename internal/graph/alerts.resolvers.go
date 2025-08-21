package graph

import (
	"context"

	"github.com/nais/api/internal/alerts"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
)

func (r *teamEnvironmentResolver) Alerts(ctx context.Context, obj *team.TeamEnvironment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *alerts.AlertOrder) (*pagination.Connection[*alerts.PrometheusAlert], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	a, err := alerts.ListPrometheusAlerts(ctx, obj.EnvironmentName, obj.TeamSlug)
	if err != nil {
		return nil, err
	}

	if orderBy == nil {
		orderBy = &alerts.AlertOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	ret := pagination.Slice(a, page)
	return pagination.NewConnection(ret, page, len(a)), nil
}
