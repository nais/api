package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/pkg/apiclient/protoapi"
)

type ReconcilerRepo interface {
	ConfigureReconciler(ctx context.Context, reconcilerName string, key string, value string) error
	DeleteReconcilerConfig(ctx context.Context, reconcilerName string, keysToDelete []string) error
	DisableReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error)
	EnableReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error)
	GetEnabledReconcilers(ctx context.Context) ([]*Reconciler, error)
	GetReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error)
	GetReconcilerConfig(ctx context.Context, reconcilerName string, includeSecrets bool) ([]*ReconcilerConfig, error)
	GetReconcilers(ctx context.Context, p Page) ([]*Reconciler, int, error)
	ResetReconcilerConfig(ctx context.Context, reconcilerName string) (*Reconciler, error)
	SyncReconcilerConfig(ctx context.Context, reconcilerName string, configs []*protoapi.ReconcilerConfigSpec) error
	UpsertReconciler(ctx context.Context, name, displayName, description string, memberAware, enableIfNew bool) (*Reconciler, error)
	UpsertReconcilerConfig(ctx context.Context, reconciler, key, displayName, description string, secret bool) error
}

var _ ReconcilerRepo = (*database)(nil)

type Reconciler struct {
	*gensql.Reconciler
}

type ReconcilerConfig struct {
	*gensql.GetReconcilerConfigRow
}

type ReconcilerError struct {
	*gensql.ReconcilerError
}

func (d *database) GetReconciler(ctx context.Context, reconcilerName string) (*Reconciler, error) {
	reconciler, err := d.querier.GetReconciler(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	return &Reconciler{Reconciler: reconciler}, nil
}

func (d *database) GetReconcilers(ctx context.Context, p Page) ([]*Reconciler, int, error) {
	rows, err := d.querier.GetReconcilers(ctx, gensql.GetReconcilersParams{
		Offset: int32(p.Offset),
		Limit:  int32(p.Limit),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := d.querier.GetReconcilersCount(ctx)
	if err != nil {
		return nil, 0, err
	}

	return wrapReconcilers(rows), int(total), nil
}

func (d *database) GetEnabledReconcilers(ctx context.Context) ([]*Reconciler, error) {
	rows, err := d.querier.GetEnabledReconcilers(ctx)
	if err != nil {
		return nil, err
	}

	return wrapReconcilers(rows), nil
}

func (d *database) ConfigureReconciler(ctx context.Context, reconcilerName string, key string, value string) error {
	return d.querier.ConfigureReconciler(ctx, gensql.ConfigureReconcilerParams{
		Value:          value,
		ReconcilerName: reconcilerName,
		Key:            key,
	})
}

func (d *database) GetReconcilerConfig(ctx context.Context, reconcilerName string, includeSecrets bool) ([]*ReconcilerConfig, error) {
	rows, err := d.querier.GetReconcilerConfig(ctx, gensql.GetReconcilerConfigParams{
		IncludeSecret:  includeSecrets,
		ReconcilerName: reconcilerName,
	})
	if err != nil {
		return nil, err
	}

	config := make([]*ReconcilerConfig, len(rows))
	for i, row := range rows {
		config[i] = &ReconcilerConfig{GetReconcilerConfigRow: row}
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

func (d *database) UpsertReconciler(ctx context.Context, name, displayName, description string, memberAware, enableIfNew bool) (*Reconciler, error) {
	reconciler, err := d.querier.UpsertReconciler(ctx, gensql.UpsertReconcilerParams{
		Name:         name,
		DisplayName:  displayName,
		Description:  description,
		MemberAware:  memberAware,
		EnabledIfNew: enableIfNew,
	})
	if err != nil {
		return nil, err
	}

	return &Reconciler{Reconciler: reconciler}, nil
}

func (d *database) UpsertReconcilerConfig(ctx context.Context, reconcilerName, key, displayName, description string, secret bool) error {
	return d.querier.UpsertReconcilerConfig(ctx, gensql.UpsertReconcilerConfigParams{
		Reconciler:  reconcilerName,
		Key:         key,
		DisplayName: displayName,
		Description: description,
		Secret:      secret,
	})
}

func (d *database) DeleteReconcilerConfig(ctx context.Context, reconcilerName string, keysToDelete []string) error {
	return d.querier.DeleteReconcilerConfig(ctx, gensql.DeleteReconcilerConfigParams{
		Reconciler: reconcilerName,
		Keys:       keysToDelete,
	})
}

func (d *database) SyncReconcilerConfig(ctx context.Context, reconcilerName string, configs []*protoapi.ReconcilerConfigSpec) error {
	cfg, err := d.GetReconcilerConfig(ctx, reconcilerName, false)
	if err != nil {
		return err
	}

	existing := make(map[string]struct{})
	for _, c := range cfg {
		existing[c.Key] = struct{}{}
	}

	return d.Transaction(ctx, func(ctx context.Context, dbtx Database) error {
		for _, c := range configs {
			if err := dbtx.UpsertReconcilerConfig(ctx, reconcilerName, c.Key, c.DisplayName, c.Description, c.Secret); err != nil {
				return err
			}
			delete(existing, c.Key)
		}

		toDelete := make([]string, len(existing))
		for k := range existing {
			toDelete = append(toDelete, k)
		}

		return dbtx.DeleteReconcilerConfig(ctx, reconcilerName, toDelete)
	})
}

func wrapReconcilers(rows []*gensql.Reconciler) []*Reconciler {
	reconcilers := make([]*Reconciler, len(rows))
	for i, row := range rows {
		reconcilers[i] = &Reconciler{Reconciler: row}
	}
	return reconcilers
}
