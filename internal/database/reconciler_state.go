package database

import (
	"context"
	"encoding/json"
	"fmt"

	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type ReconcilerStateRepo interface {
	LoadReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug, state interface{}) error
	RemoveReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug) error
	SetReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug, state interface{}) error
}

// LoadReconcilerStateForTeam Load the team state for a given reconciler into the state parameter
func (d *database) LoadReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug, state interface{}) error {
	systemState, err := d.querier.GetReconcilerStateForTeam(ctx, reconcilerName, slug)
	if err != nil {
		// assume empty state
		systemState = &sqlc.ReconcilerState{State: []byte{}}
	}

	err = json.Unmarshal(systemState.State, state)
	if err != nil {
		return fmt.Errorf("unable to assign state: %w", err)
	}

	return nil
}

// SetReconcilerStateForTeam Update the team state for a given reconciler
func (d *database) SetReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug, state interface{}) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("unable to set new system state: %w", err)
	}

	return d.querier.SetReconcilerStateForTeam(ctx, reconcilerName, slug, data)
}

func (d *database) RemoveReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug) error {
	return d.querier.RemoveReconcilerStateForTeam(ctx, reconcilerName, slug)
}
