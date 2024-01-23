package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

type ReconcilerErrorRepo interface {
	ClearReconcilerErrorsForTeam(ctx context.Context, slug slug.Slug, reconcilerName string) error
	GetTeamReconcilerErrors(ctx context.Context, slug slug.Slug) ([]*ReconcilerError, error)
	SetReconcilerErrorForTeam(ctx context.Context, correlationID uuid.UUID, slug slug.Slug, reconcilerName string, err error) error
}

func (d *database) SetReconcilerErrorForTeam(ctx context.Context, correlationID uuid.UUID, slug slug.Slug, reconcilerName string, err error) error {
	return d.querier.SetReconcilerErrorForTeam(ctx, correlationID, slug, reconcilerName, err.Error())
}

func (d *database) GetTeamReconcilerErrors(ctx context.Context, slug slug.Slug) ([]*ReconcilerError, error) {
	rows, err := d.querier.GetTeamReconcilerErrors(ctx, slug)
	if err != nil {
		return nil, err
	}

	errors := make([]*ReconcilerError, 0)
	for _, row := range rows {
		errors = append(errors, &ReconcilerError{ReconcilerError: row})
	}

	return errors, nil
}

func (d *database) ClearReconcilerErrorsForTeam(ctx context.Context, slug slug.Slug, reconcilerName string) error {
	return d.querier.ClearReconcilerErrorsForTeam(ctx, slug, reconcilerName)
}
