package database

import (
	"context"

	"github.com/google/uuid"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type ReconcilerRepo interface {
	AddReconcilerOptOut(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, reconcilerName string) error
	ConfigureReconciler(ctx context.Context, reconcilerName string, key string, value string) error
	DangerousGetReconcilerConfigValues(ctx context.Context, reconcilerName string) (*ReconcilerConfigValues, error)
	DisableReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error)
	EnableReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error)
	GetEnabledReconcilers(ctx context.Context) ([]*Reconciler, error)
	GetReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error)
	GetReconcilerConfig(ctx context.Context, reconcilerName string) ([]*ReconcilerConfig, error)
	GetReconcilers(ctx context.Context) ([]*Reconciler, error)
	RemoveReconcilerOptOut(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, reconcilerName string) error
	ResetReconcilerConfig(ctx context.Context, reconcilerName string) (*Reconciler, error)
	UpsertReconciler(ctx context.Context, name, display_name, description string, memberAware bool) (*Reconciler, error)
}

type Reconciler struct {
	*sqlc.Reconciler
}

type ReconcilerConfig struct {
	*sqlc.GetReconcilerConfigRow
}

type ReconcilerError struct {
	*sqlc.ReconcilerError
}

type ReconcilerConfigValues struct {
	values map[string]string
}

func (v ReconcilerConfigValues) GetValue(s string) string {
	if v, exists := v.values[s]; exists {
		return v
	}
	return ""
}

func (d *database) GetReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error) {
	reconciler, err := d.querier.GetReconciler(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	return &Reconciler{Reconciler: reconciler}, nil
}

func (d *database) GetReconcilers(ctx context.Context) ([]*Reconciler, error) {
	rows, err := d.querier.GetReconcilers(ctx)
	if err != nil {
		return nil, err
	}

	return wrapReconcilers(rows), nil
}

func (d *database) GetEnabledReconcilers(ctx context.Context) ([]*Reconciler, error) {
	rows, err := d.querier.GetEnabledReconcilers(ctx)
	if err != nil {
		return nil, err
	}

	return wrapReconcilers(rows), nil
}

func (d *database) ConfigureReconciler(ctx context.Context, reconcilerName string, key string, value string) error {
	return d.querier.ConfigureReconciler(ctx, value, reconcilerName, key)
}

func (d *database) GetReconcilerConfig(ctx context.Context, reconcilerName string) ([]*ReconcilerConfig, error) {
	rows, err := d.querier.GetReconcilerConfig(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	config := make([]*ReconcilerConfig, 0, len(rows))
	for _, row := range rows {
		config = append(config, &ReconcilerConfig{GetReconcilerConfigRow: row})
	}

	return config, nil
}

func (d *database) ResetReconcilerConfig(ctx context.Context, reconcilerName string) (*Reconciler, error) {
	reconciler, err := d.querier.GetReconciler(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	err = d.querier.ResetReconcilerConfig(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	return &Reconciler{Reconciler: reconciler}, nil
}

func (d *database) EnableReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error) {
	reconciler, err := d.querier.EnableReconciler(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	return &Reconciler{Reconciler: reconciler}, nil
}

func (d *database) DisableReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error) {
	reconciler, err := d.querier.DisableReconciler(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	return &Reconciler{Reconciler: reconciler}, nil
}

func (d *database) DangerousGetReconcilerConfigValues(ctx context.Context, reconcilerName string) (*ReconcilerConfigValues, error) {
	rows, err := d.querier.DangerousGetReconcilerConfigValues(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string)
	for _, row := range rows {
		values[row.Key] = row.Value
	}

	return &ReconcilerConfigValues{values: values}, nil
}

func (d *database) AddReconcilerOptOut(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, reconcilerName string) error {
	return d.querier.AddReconcilerOptOut(ctx, teamSlug, userID, reconcilerName)
}

func (d *database) RemoveReconcilerOptOut(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, reconcilerName string) error {
	return d.querier.RemoveReconcilerOptOut(ctx, teamSlug, userID, reconcilerName)
}

func (d *database) UpsertReconciler(ctx context.Context, name, display_name, description string, memberAware bool) (*Reconciler, error) {
	reconciler, err := d.querier.UpsertReconciler(ctx, name, display_name, description, memberAware)
	if err != nil {
		return nil, err
	}

	return &Reconciler{Reconciler: reconciler}, nil
}

func wrapReconcilers(rows []*sqlc.Reconciler) []*Reconciler {
	reconcilers := make([]*Reconciler, 0, len(rows))
	for _, row := range rows {
		reconcilers = append(reconcilers, &Reconciler{Reconciler: row})
	}
	return reconcilers
}
