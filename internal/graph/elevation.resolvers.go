package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/elevation"
	"github.com/nais/api/internal/graph/gengql"
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

func (r *queryResolver) Elevations(ctx context.Context, input elevation.ElevationInput) ([]*elevation.Elevation, error) {
	actor := authz.ActorFromContext(ctx)
	return elevation.List(ctx, &input, actor)
}

func (r *Resolver) ElevationCreatedActivityLogEntry() gengql.ElevationCreatedActivityLogEntryResolver {
	return &elevationCreatedActivityLogEntryResolver{r}
}

type elevationCreatedActivityLogEntryResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
/*
	func (r *elevationResolver) ID(ctx context.Context, obj *elevation.Elevation) (*ident.Ident, error) {
	return &obj.ID, nil
}
func (r *Resolver) Elevation() gengql.ElevationResolver { return &elevationResolver{r} }
type (
	elevationResolver                        struct{ *Resolver }
	elevationCreatedActivityLogEntryResolver struct{ *Resolver }
)
*/
