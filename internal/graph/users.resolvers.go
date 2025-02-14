package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/user"
)

func (r *queryResolver) Users(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *user.UserOrder) (*pagination.Connection[*user.User], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return user.List(ctx, page, orderBy)
}

func (r *queryResolver) User(ctx context.Context, email *string) (*user.User, error) {
	if email == nil {
		return nil, apierror.Errorf("email argument must be provided")
	}
	return user.GetByEmail(ctx, *email)
}

func (r *queryResolver) Me(ctx context.Context) (authz.AuthenticatedUser, error) {
	return authz.ActorFromContext(ctx).User, nil
}

func (r *userResolver) Teams(ctx context.Context, obj *user.User, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *team.UserTeamOrder) (*pagination.Connection[*team.TeamMember], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return team.ListForUser(ctx, obj.UUID, page, orderBy)
}

func (r *Resolver) User() gengql.UserResolver { return &userResolver{r} }

type userResolver struct{ *Resolver }
