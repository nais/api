package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/reconciler"
	"github.com/nais/api/internal/team"
)

func (r *mutationResolver) EnableReconciler(ctx context.Context, input reconciler.EnableReconcilerInput) (*reconciler.Reconciler, error) {
	if err := authz.RequireGlobalAdmin(ctx); err != nil {
		return nil, err
	}

	rec, err := reconciler.Enable(ctx, input.Name)
	if err != nil {
		return nil, err
	}

	r.triggerReconcilerEnabledEvent(ctx, rec.Name, uuid.Nil)
	return rec, nil
}

func (r *mutationResolver) DisableReconciler(ctx context.Context, input reconciler.DisableReconcilerInput) (*reconciler.Reconciler, error) {
	if err := authz.RequireGlobalAdmin(ctx); err != nil {
		return nil, err
	}

	rec, err := reconciler.Disable(ctx, input.Name)
	if err != nil {
		return nil, err
	}

	r.triggerReconcilerDisabledEvent(ctx, rec.Name, uuid.Nil)
	return rec, nil
}

func (r *mutationResolver) ConfigureReconciler(ctx context.Context, input reconciler.ConfigureReconcilerInput) (*reconciler.Reconciler, error) {
	if err := authz.RequireGlobalAdmin(ctx); err != nil {
		return nil, err
	}

	rec, err := reconciler.Configure(ctx, input.Name, input.Config)
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
	if err := authz.RequireGlobalAdmin(ctx); err != nil {
		return nil, err
	}
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
	if err := authz.RequireGlobalAdmin(ctx); err != nil {
		return nil, err
	}
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return reconciler.GetErrors(ctx, obj.Name, page)
}

func (r *reconcilerErrorResolver) Team(ctx context.Context, obj *reconciler.ReconcilerError) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *Resolver) Reconciler() gengql.ReconcilerResolver { return &reconcilerResolver{r} }

func (r *Resolver) ReconcilerError() gengql.ReconcilerErrorResolver {
	return &reconcilerErrorResolver{r}
}

type reconcilerResolver struct{ *Resolver }
type reconcilerErrorResolver struct{ *Resolver }
