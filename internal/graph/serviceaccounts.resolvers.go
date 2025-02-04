package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/team"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) CreateServiceAccount(ctx context.Context, input serviceaccount.CreateServiceAccountInput) (*serviceaccount.CreateServiceAccountPayload, error) {
	sa, err := serviceaccount.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	return &serviceaccount.CreateServiceAccountPayload{ServiceAccount: sa}, nil
}

func (r *mutationResolver) UpdateServiceAccount(ctx context.Context, input serviceaccount.UpdateServiceAccountInput) (*serviceaccount.UpdateServiceAccountPayload, error) {
	sa, err := serviceaccount.Update(ctx, input)
	if err != nil {
		return nil, err
	}

	return &serviceaccount.UpdateServiceAccountPayload{ServiceAccount: sa}, nil
}

func (r *mutationResolver) DeleteServiceAccount(ctx context.Context, input serviceaccount.DeleteServiceAccountInput) (*serviceaccount.DeleteServiceAccountPayload, error) {
	err := serviceaccount.Delete(ctx, input)
	if err != nil {
		return nil, err
	}

	return &serviceaccount.DeleteServiceAccountPayload{ServiceAccountDeleted: ptr.To(true)}, nil
}

func (r *mutationResolver) AddRoleToServiceAccount(ctx context.Context, input serviceaccount.AddRoleToServiceAccountInput) (*serviceaccount.AddRoleToServiceAccountPayload, error) {
	panic(fmt.Errorf("not implemented: AddRoleToServiceAccount - addRoleToServiceAccount"))
}

func (r *mutationResolver) RemoveRoleFromServiceAccount(ctx context.Context, input serviceaccount.RemoveRoleFromServiceAccountInput) (*serviceaccount.RemoveRoleFromServiceAccountPayload, error) {
	panic(fmt.Errorf("not implemented: RemoveRoleFromServiceAccount - removeRoleFromServiceAccount"))
}

func (r *mutationResolver) CreateServiceAccountToken(ctx context.Context, input serviceaccount.CreateServiceAccountTokenInput) (*serviceaccount.CreateServiceAccountTokenPayload, error) {
	panic(fmt.Errorf("not implemented: CreateServiceAccountToken - createServiceAccountToken"))
}

func (r *mutationResolver) UpdateServiceAccountToken(ctx context.Context, input serviceaccount.UpdateServiceAccountTokenInput) (*serviceaccount.UpdateServiceAccountTokenPayload, error) {
	panic(fmt.Errorf("not implemented: UpdateServiceAccountToken - updateServiceAccountToken"))
}

func (r *mutationResolver) DeleteServiceAccountToken(ctx context.Context, input serviceaccount.DeleteServiceAccountTokenInput) (*serviceaccount.DeleteServiceAccountTokenPayload, error) {
	panic(fmt.Errorf("not implemented: DeleteServiceAccountToken - deleteServiceAccountToken"))
}

func (r *queryResolver) ServiceAccounts(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*serviceaccount.ServiceAccount], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return serviceaccount.List(ctx, page)
}

func (r *queryResolver) ServiceAccount(ctx context.Context, id ident.Ident) (*serviceaccount.ServiceAccount, error) {
	return serviceaccount.GetByIdent(ctx, id)
}

func (r *serviceAccountResolver) Team(ctx context.Context, obj *serviceaccount.ServiceAccount) (*team.Team, error) {
	if obj.TeamSlug == nil {
		return nil, nil
	}

	return team.Get(ctx, *obj.TeamSlug)
}

func (r *Resolver) ServiceAccount() gengql.ServiceAccountResolver { return &serviceAccountResolver{r} }

type serviceAccountResolver struct{ *Resolver }
