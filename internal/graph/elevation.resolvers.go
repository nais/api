package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/elevation"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

func (r *elevationCreatedActivityLogEntryResolver) ElevationType(ctx context.Context, obj *elevation.ElevationCreatedActivityLogEntry) (elevation.ElevationType, error) {
	return elevation.ElevationType(obj.GetElevationType()), nil
}

func (r *elevationCreatedActivityLogEntryResolver) TargetResourceName(ctx context.Context, obj *elevation.ElevationCreatedActivityLogEntry) (string, error) {
	return obj.GetTargetResourceName(), nil
}

func (r *elevationCreatedActivityLogEntryResolver) Reason(ctx context.Context, obj *elevation.ElevationCreatedActivityLogEntry) (string, error) {
	return obj.GetReason(), nil
}

func (r *elevationCreatedActivityLogEntryResolver) ExpiresAt(ctx context.Context, obj *elevation.ElevationCreatedActivityLogEntry) (*time.Time, error) {
	expiresAtStr := obj.GetExpiresAt()
	if expiresAtStr == "" {
		return nil, nil
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing expiresAt: %w", err)
	}

	return &expiresAt, nil
}

func (r *mutationResolver) CreateElevation(ctx context.Context, input elevation.CreateElevationInput) (*elevation.CreateElevationPayload, error) {
	actor := authz.ActorFromContext(ctx)

	elev, err := elevation.Create(ctx, &input, actor)
	if err != nil {
		return nil, err
	}

	return &elevation.CreateElevationPayload{
		Elevation: elev,
	}, nil
}

func (r *mutationResolver) RevokeElevation(ctx context.Context, input elevation.RevokeElevationInput) (*elevation.RevokeElevationPayload, error) {
	actor := authz.ActorFromContext(ctx)

	if err := elevation.Revoke(ctx, &input, actor); err != nil {
		return nil, err
	}

	return &elevation.RevokeElevationPayload{
		Success: true,
	}, nil
}

func (r *queryResolver) MyElevations(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*elevation.Elevation], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx)

	elevations, err := elevation.ListForUser(ctx, actor)
	if err != nil {
		return nil, err
	}

	ret := pagination.Slice(elevations, page)
	return pagination.NewConnection(ret, page, len(elevations)), nil
}

func (r *queryResolver) Elevations(ctx context.Context, team slug.Slug, environment string, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, includeHistory *bool) (*pagination.Connection[*elevation.Elevation], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	// Check team authorization
	if err := authz.CanUpdateTeamMetadata(ctx, team); err != nil {
		return nil, elevation.ErrNotAuthorized
	}

	elevations, err := elevation.ListForTeamEnvironment(ctx, team, environment)
	if err != nil {
		return nil, err
	}

	// TODO: if includeHistory is true, also fetch from activity log

	ret := pagination.Slice(elevations, page)
	return pagination.NewConnection(ret, page, len(elevations)), nil
}

func (r *queryResolver) Elevation(ctx context.Context, id ident.Ident) (*elevation.Elevation, error) {
	return elevation.Get(ctx, id.ID)
}

func (r *Resolver) ElevationCreatedActivityLogEntry() gengql.ElevationCreatedActivityLogEntryResolver {
	return &elevationCreatedActivityLogEntryResolver{r}
}

type elevationCreatedActivityLogEntryResolver struct{ *Resolver }
