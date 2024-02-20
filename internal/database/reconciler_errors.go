package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type ReconcilerErrorRepo interface {
	ClearReconcilerErrorsForTeam(ctx context.Context, teamSlug slug.Slug, reconcilerName string) error
	GetTeamReconcilerErrors(ctx context.Context, teamSlug slug.Slug) ([]*ReconcilerError, error)
	SetReconcilerErrorForTeam(ctx context.Context, correlationID uuid.UUID, teamSlug slug.Slug, reconcilerName string, err error) error
}

func (d *database) SetReconcilerErrorForTeam(ctx context.Context, correlationID uuid.UUID, teamSlug slug.Slug, reconcilerName string, err error) error {
	return d.querier.SetReconcilerErrorForTeam(ctx, gensql.SetReconcilerErrorForTeamParams{
		CorrelationID: correlationID,
		TeamSlug:      teamSlug,
		Reconciler:    reconcilerName,
		ErrorMessage:  err.Error(),
	})
}

func (d *database) GetTeamReconcilerErrors(ctx context.Context, teamSlug slug.Slug) ([]*ReconcilerError, error) {
	rows, err := d.querier.GetTeamReconcilerErrors(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	ret := make([]*ReconcilerError, len(rows))
	for i, row := range rows {
		ret[i] = &ReconcilerError{ReconcilerError: row}
	}

	return ret, nil
}

func (d *database) ClearReconcilerErrorsForTeam(ctx context.Context, teamSlug slug.Slug, reconcilerName string) error {
	return d.querier.ClearReconcilerErrorsForTeam(ctx, gensql.ClearReconcilerErrorsForTeamParams{
		TeamSlug:   teamSlug,
		Reconciler: reconcilerName,
	})
}
