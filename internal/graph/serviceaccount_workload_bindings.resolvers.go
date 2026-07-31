package graph

import (
	"context"
	"errors"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/workload"
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
	return workloadOrNil(ctx, obj.TeamSlug, obj.Environment, obj.WorkloadName), nil
}

func (r *serviceAccountWorkloadBindingResolver) IsBroken(ctx context.Context, obj *serviceaccount.ServiceAccountWorkloadBinding) (bool, error) {
	w, err := r.Workload(ctx, obj)
	if err != nil && !errors.Is(err, &watcher.ErrorNotFound{}) {
		return false, err
	}
	return w == nil, nil
}

func (r *serviceAccountWorkloadBindingAddedActivityLogEntryDataResolver) Workload(ctx context.Context, obj *serviceaccount.ServiceAccountWorkloadBindingAddedActivityLogEntryData) (workload.Workload, error) {
	return workloadOrNil(ctx, obj.TeamSlug, obj.Environment, obj.WorkloadName), nil
}

func (r *serviceAccountWorkloadBindingRemovedActivityLogEntryDataResolver) Workload(ctx context.Context, obj *serviceaccount.ServiceAccountWorkloadBindingRemovedActivityLogEntryData) (workload.Workload, error) {
	return workloadOrNil(ctx, obj.TeamSlug, obj.Environment, obj.WorkloadName), nil
}

func (r *Resolver) ServiceAccountWorkloadBinding() gengql.ServiceAccountWorkloadBindingResolver {
	return &serviceAccountWorkloadBindingResolver{r}
}

func (r *Resolver) ServiceAccountWorkloadBindingAddedActivityLogEntryData() gengql.ServiceAccountWorkloadBindingAddedActivityLogEntryDataResolver {
	return &serviceAccountWorkloadBindingAddedActivityLogEntryDataResolver{r}
}

func (r *Resolver) ServiceAccountWorkloadBindingRemovedActivityLogEntryData() gengql.ServiceAccountWorkloadBindingRemovedActivityLogEntryDataResolver {
	return &serviceAccountWorkloadBindingRemovedActivityLogEntryDataResolver{r}
}

type (
	serviceAccountWorkloadBindingResolver                            struct{ *Resolver }
	serviceAccountWorkloadBindingAddedActivityLogEntryDataResolver   struct{ *Resolver }
	serviceAccountWorkloadBindingRemovedActivityLogEntryDataResolver struct{ *Resolver }
)
