package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/team"
)

func (r *mutationResolver) CreateServiceAccount(ctx context.Context, input serviceaccount.CreateServiceAccountInput) (*serviceaccount.ServiceAccount, error) {
	actor := authz.ActorFromContext(ctx)

	if input.TeamSlug == nil {
		err := authz.RequireGlobalAuthorization(actor, role.AuthorizationServiceAccountsCreate)
		if err != nil {
			return nil, err
		}
	} else {
		err := authz.RequireTeamAuthorization(actor, role.AuthorizationServiceAccountsCreate, *input.TeamSlug)
		if err != nil {
			return nil, err
		}
	}

	return serviceaccount.Create(ctx, input)
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
