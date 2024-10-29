package graphv1

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/reconciler"
	"github.com/nais/api/internal/v1/team"
)

func (r *mutationResolver) EnableReconciler(ctx context.Context, name string) (*reconciler.Reconciler, error) {
	rec, err := reconciler.Enable(ctx, name)
	if err != nil {
		return nil, err
	}

	r.triggerReconcilerEnabledEvent(ctx, rec.Name, uuid.Nil)
	return rec, nil
}

func (r *mutationResolver) DisableReconciler(ctx context.Context, name string) (*reconciler.Reconciler, error) {
	rec, err := reconciler.Disable(ctx, name)
	if err != nil {
		return nil, err
	}

	r.triggerReconcilerDisabledEvent(ctx, rec.Name, uuid.Nil)
	return rec, nil
}

func (r *mutationResolver) ConfigureReconciler(ctx context.Context, name string, config []*reconciler.ReconcilerConfigInput) (*reconciler.Reconciler, error) {
	rec, err := reconciler.Configure(ctx, name, config)
	if err != nil {
		return nil, err
	}

	r.triggerReconcilerConfiguredEvent(ctx, rec.Name, uuid.Nil)
	return rec, nil
}

func (r *queryResolver) Reconcilers(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*reconciler.Reconciler], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return reconciler.List(ctx, page)
}

func (r *reconcilerResolver) Config(ctx context.Context, obj *reconciler.Reconciler) ([]*reconciler.ReconcilerConfig, error) {
	return reconciler.GetConfig(ctx, obj.Name, false)
}

func (r *reconcilerResolver) Configured(ctx context.Context, obj *reconciler.Reconciler) (bool, error) {
	configs, err := reconciler.GetConfig(ctx, obj.Name, false)
	if err != nil {
		return false, err
	}

	for _, config := range configs {
		if !config.Configured {
			return false, nil
		}
	}

	return true, nil
}

func (r *reconcilerResolver) Errors(ctx context.Context, obj *reconciler.Reconciler, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*reconciler.ReconcilerError], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return reconciler.GetErrors(ctx, obj.Name, page)
}

func (r *reconcilerErrorResolver) Team(ctx context.Context, obj *reconciler.ReconcilerError) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *Resolver) Reconciler() gengqlv1.ReconcilerResolver { return &reconcilerResolver{r} }

func (r *Resolver) ReconcilerError() gengqlv1.ReconcilerErrorResolver {
	return &reconcilerErrorResolver{r}
}

type (
	reconcilerResolver      struct{ *Resolver }
	reconcilerErrorResolver struct{ *Resolver }
)
