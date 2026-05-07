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
	// TODO(thokra): Broken if the workload no longer exists. We can't detect UID mismatch from a stored binding alone — that
	// only surfaces at authentication time when the actual UID does not match the pinned one. Once such an
	// authentication has been attempted, future requests will keep failing; surfacing UID mismatch here would
	// require additional bookkeeping which is out of scope for now.
	return w == nil, nil
}

func (r *Resolver) ServiceAccountWorkloadBinding() gengql.ServiceAccountWorkloadBindingResolver {
	return &serviceAccountWorkloadBindingResolver{r}
}

type serviceAccountWorkloadBindingResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
/*
	func (r *serviceAccountWorkloadBindingAddedActivityLogEntryDataResolver) TeamSlug(ctx context.Context, obj *serviceaccount.ServiceAccountWorkloadBindingAddedActivityLogEntryData) (slug.Slug, error) {
	panic(fmt.Errorf("not implemented: TeamSlug - teamSlug"))
}
func (r *serviceAccountWorkloadBindingRemovedActivityLogEntryDataResolver) TeamSlug(ctx context.Context, obj *serviceaccount.ServiceAccountWorkloadBindingRemovedActivityLogEntryData) (slug.Slug, error) {
	panic(fmt.Errorf("not implemented: TeamSlug - teamSlug"))
}
func (r *Resolver) ServiceAccountWorkloadBindingAddedActivityLogEntryData() gengql.ServiceAccountWorkloadBindingAddedActivityLogEntryDataResolver {
	return &serviceAccountWorkloadBindingAddedActivityLogEntryDataResolver{r}
}
func (r *Resolver) ServiceAccountWorkloadBindingRemovedActivityLogEntryData() gengql.ServiceAccountWorkloadBindingRemovedActivityLogEntryDataResolver {
	return &serviceAccountWorkloadBindingRemovedActivityLogEntryDataResolver{r}
}
*/
