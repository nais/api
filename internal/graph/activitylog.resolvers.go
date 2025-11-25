package graph

import (
	"context"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/reconciler"
	"github.com/nais/api/internal/team"
)

func (r *openSearchResolver) ActivityLog(ctx context.Context, obj *opensearch.OpenSearch, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*pagination.Connection[activitylog.ActivityLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return activitylog.ListForResourceTeamAndEnvironment(
		ctx,
		opensearch.ActivityLogEntryResourceTypeOpenSearch,
		obj.TeamSlug,
		obj.Name,
		environmentmapper.EnvironmentName(obj.EnvironmentName),
		page,
		filter,
	)
}

func (r *reconcilerResolver) ActivityLog(ctx context.Context, obj *reconciler.Reconciler, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*pagination.Connection[activitylog.ActivityLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return activitylog.ListForResource(ctx, reconciler.ActivityLogEntryResourceTypeReconciler, obj.Name, page, filter)
}

func (r *teamResolver) ActivityLog(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*pagination.Connection[activitylog.ActivityLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return activitylog.ListForTeam(ctx, obj.Slug, page, filter)
}

func (r *valkeyResolver) ActivityLog(ctx context.Context, obj *valkey.Valkey, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*pagination.Connection[activitylog.ActivityLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return activitylog.ListForResourceTeamAndEnvironment(
		ctx,
		valkey.ActivityLogEntryResourceTypeValkey,
		obj.TeamSlug,
		obj.Name,
		environmentmapper.EnvironmentName(obj.EnvironmentName),
		page,
		filter,
	)
}
