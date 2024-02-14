package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type ReconcilerState struct {
	*gensql.ReconcilerState
}

type ReconcilerStateRepo interface {
	GetReconcilerStateForTeam(ctx context.Context, reconcilerName string, teamSlug slug.Slug) (*ReconcilerState, error)
	UpsertReconcilerState(ctx context.Context, reconcilerName string, teamSlug slug.Slug, value []byte) (*ReconcilerState, error)
	DeleteReconcilerStateForTeam(ctx context.Context, reconcilerName string, teamSlug slug.Slug) error
}

func (d *database) GetReconcilerStateForTeam(ctx context.Context, reconcilerName string, teamSlug slug.Slug) (*ReconcilerState, error) {
	row, err := d.querier.GetReconcilerStateForTeam(ctx, gensql.GetReconcilerStateForTeamParams{
		ReconcilerName: reconcilerName,
		TeamSlug:       teamSlug,
	})
	if err != nil {
		return nil, err
	}

	return &ReconcilerState{row}, nil
}

func (d *database) UpsertReconcilerState(ctx context.Context, reconcilerName string, teamSlug slug.Slug, value []byte) (*ReconcilerState, error) {
	row, err := d.querier.UpsertReconcilerState(ctx, gensql.UpsertReconcilerStateParams{
		ReconcilerName: reconcilerName,
		TeamSlug:       teamSlug,
		Value:          value,
	})
	if err != nil {
		return nil, err
	}

	return &ReconcilerState{row}, nil
}

func (d *database) DeleteReconcilerStateForTeam(ctx context.Context, reconcilerName string, teamSlug slug.Slug) error {
	return d.querier.DeleteReconcilerStateForTeam(ctx, gensql.DeleteReconcilerStateForTeamParams{
		ReconcilerName: reconcilerName,
		TeamSlug:       teamSlug,
	})
}
