package graph

import (
	"context"
	"errors"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *mutationResolver) AddWorkloadToServiceAccount(ctx context.Context, input serviceaccount.AddWorkloadToServiceAccountInput) (*serviceaccount.AddWorkloadToServiceAccountPayload, error) {
	sa, binding, err := serviceaccount.AddWorkloadBinding(ctx, input)
	if err != nil {
		return nil, err
	}
	return &serviceaccount.AddWorkloadToServiceAccountPayload{
		ServiceAccount: sa,
		Binding:        binding,
	}, nil
}

func (r *mutationResolver) RemoveWorkloadFromServiceAccount(ctx context.Context, input serviceaccount.RemoveWorkloadFromServiceAccountInput) (*serviceaccount.RemoveWorkloadFromServiceAccountPayload, error) {
	sa, err := serviceaccount.RemoveWorkloadBinding(ctx, input)
	if err != nil {
		return nil, err
	}
	return &serviceaccount.RemoveWorkloadFromServiceAccountPayload{
		ServiceAccount: sa,
		BindingDeleted: new(true),
	}, nil
}

func (r *serviceAccountResolver) WorkloadBindings(ctx context.Context, obj *serviceaccount.ServiceAccount, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*serviceaccount.ServiceAccountWorkloadBinding], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return serviceaccount.ListBindingsForServiceAccount(ctx, page, obj.UUID)
}

func (r *serviceAccountWorkloadBindingResolver) ServiceAccount(ctx context.Context, obj *serviceaccount.ServiceAccountWorkloadBinding) (*serviceaccount.ServiceAccount, error) {
	return serviceaccount.Get(ctx, obj.ServiceAccountID)
}

func (r *serviceAccountWorkloadBindingResolver) Workload(ctx context.Context, obj *serviceaccount.ServiceAccountWorkloadBinding) (workload.Workload, error) {
	if app, err := application.Get(ctx, obj.TeamSlug, obj.Environment, obj.WorkloadName); err == nil {
		return app, nil
	}
	if j, err := job.Get(ctx, obj.TeamSlug, obj.Environment, obj.WorkloadName); err == nil {
		return j, nil
	}
	// No matching workload — binding is dangling.
	return nil, nil
}

func (r *serviceAccountWorkloadBindingResolver) IsBroken(ctx context.Context, obj *serviceaccount.ServiceAccountWorkloadBinding) (bool, error) {
	w, err := r.Workload(ctx, obj)
	if err != nil && !errors.Is(err, &watcher.ErrorNotFound{}) {
		return false, err
	}
	return w == nil, nil
}

func (r *Resolver) ServiceAccountWorkloadBinding() gengql.ServiceAccountWorkloadBindingResolver {
	return &serviceAccountWorkloadBindingResolver{r}
}

type serviceAccountWorkloadBindingResolver struct{ *Resolver }
