package db

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/slug"

	"github.com/jackc/pgtype"
	"github.com/nais/api/internal/sqlc"
)

// LoadReconcilerStateForTeam Load the team state for a given reconciler into the state parameter
func (d *database) LoadReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug, state interface{}) error {
	systemState, err := d.querier.GetReconcilerStateForTeam(ctx, sqlc.GetReconcilerStateForTeamParams{
		Reconciler: reconcilerName,
		TeamSlug:   slug,
	})
	if err != nil {
		// assume empty state
		systemState = &sqlc.ReconcilerState{State: pgtype.JSONB{}}
	}

	err = systemState.State.AssignTo(state)
	if err != nil {
		return fmt.Errorf("unable to assign state: %w", err)
	}

	return nil
}

// SetReconcilerStateForTeam Update the team state for a given reconciler
func (d *database) SetReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug, state interface{}) error {
	newState := pgtype.JSONB{}
	err := newState.Set(state)
	if err != nil {
		return fmt.Errorf("unable to set new system state: %w", err)
	}

	return d.querier.SetReconcilerStateForTeam(ctx, sqlc.SetReconcilerStateForTeamParams{
		Reconciler: reconcilerName,
		TeamSlug:   slug,
		State:      newState,
	})
}

func (d *database) RemoveReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug) error {
	return d.querier.RemoveReconcilerStateForTeam(ctx, sqlc.RemoveReconcilerStateForTeamParams{
		Reconciler: reconcilerName,
		TeamSlug:   slug,
	})
}
