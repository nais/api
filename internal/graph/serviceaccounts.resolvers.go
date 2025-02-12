package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
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

func (r *mutationResolver) AssignRoleToServiceAccount(ctx context.Context, input serviceaccount.AssignRoleToServiceAccountInput) (*serviceaccount.AssignRoleToServiceAccountPayload, error) {
	sa, err := serviceaccount.AssignRole(ctx, input)
	if err != nil {
		return nil, err
	}

	return &serviceaccount.AssignRoleToServiceAccountPayload{ServiceAccount: sa}, nil
}

func (r *mutationResolver) RevokeRoleFromServiceAccount(ctx context.Context, input serviceaccount.RevokeRoleFromServiceAccountInput) (*serviceaccount.RevokeRoleFromServiceAccountPayload, error) {
	sa, err := serviceaccount.RevokeRole(ctx, input)
	if err != nil {
		return nil, err
	}

	return &serviceaccount.RevokeRoleFromServiceAccountPayload{ServiceAccount: sa}, nil
}

func (r *mutationResolver) CreateServiceAccountToken(ctx context.Context, input serviceaccount.CreateServiceAccountTokenInput) (*serviceaccount.CreateServiceAccountTokenPayload, error) {
	sa, token, secret, err := serviceaccount.CreateToken(ctx, input)
	if err != nil {
		return nil, err
	}

	return &serviceaccount.CreateServiceAccountTokenPayload{
		ServiceAccountToken: token,
		ServiceAccount:      sa,
		Secret:              secret,
	}, nil
}

func (r *mutationResolver) UpdateServiceAccountToken(ctx context.Context, input serviceaccount.UpdateServiceAccountTokenInput) (*serviceaccount.UpdateServiceAccountTokenPayload, error) {
	sa, token, err := serviceaccount.UpdateToken(ctx, input)
	if err != nil {
		return nil, err
	}

	return &serviceaccount.UpdateServiceAccountTokenPayload{
		ServiceAccountToken: token,
		ServiceAccount:      sa,
	}, nil
}

func (r *mutationResolver) DeleteServiceAccountToken(ctx context.Context, input serviceaccount.DeleteServiceAccountTokenInput) (*serviceaccount.DeleteServiceAccountTokenPayload, error) {
	sa, err := serviceaccount.DeleteToken(ctx, input)
	if err != nil {
		return nil, err
	}

	return &serviceaccount.DeleteServiceAccountTokenPayload{
		ServiceAccountTokenDeleted: ptr.To(true),
		ServiceAccount:             sa,
	}, nil
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

func (r *serviceAccountResolver) Roles(ctx context.Context, obj *serviceaccount.ServiceAccount, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*authz.Role], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return authz.ListRolesForServiceAccount(ctx, obj.UUID, page)
}

func (r *serviceAccountResolver) Tokens(ctx context.Context, obj *serviceaccount.ServiceAccount, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*serviceaccount.ServiceAccountToken], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return serviceaccount.ListTokensForServiceAccount(ctx, page, obj.UUID)
}

func (r *Resolver) ServiceAccount() gengql.ServiceAccountResolver { return &serviceAccountResolver{r} }

type serviceAccountResolver struct{ *Resolver }
