package graph

import (
	"context"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/reconciler"
	"github.com/nais/api/internal/team"
)

func (r *reconcilerResolver) AuditEntries(ctx context.Context, obj *reconciler.Reconciler, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[activitylog.AuditEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return activitylog.ListForResource(ctx, reconciler.AuditResourceTypeReconciler, obj.Name, page)
}

func (r *teamResolver) AuditEntries(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[activitylog.AuditEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return activitylog.ListForTeam(ctx, obj.Slug, page)
}
