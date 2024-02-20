package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type ReconcilerState struct {
	*gensql.ReconcilerState
}

type ReconcilerStateWithTeam struct {
	*Team
	*gensql.ReconcilerState
}

type ReconcilerStateRepo interface {
	GetReconcilerState(ctx context.Context, reconcilerName string) ([]*ReconcilerStateWithTeam, error)
	GetReconcilerStateForTeam(ctx context.Context, reconcilerName string, teamSlug slug.Slug) (*ReconcilerState, error)
	UpsertReconcilerState(ctx context.Context, reconcilerName string, teamSlug slug.Slug, value []byte) (*ReconcilerState, error)
	DeleteReconcilerStateForTeam(ctx context.Context, reconcilerName string, teamSlug slug.Slug) error
}

func (d *database) GetReconcilerState(ctx context.Context, reconcilerName string) ([]*ReconcilerStateWithTeam, error) {
	rows, err := d.querier.GetReconcilerState(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}
	state := make([]*ReconcilerStateWithTeam, len(rows))
	for i, row := range rows {
		state[i] = &ReconcilerStateWithTeam{
			ReconcilerState: &row.ReconcilerState,
			Team:            &Team{&row.Team},
		}
	}
	return state, nil
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
