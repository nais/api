package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/pagination"
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

func (r *queryResolver) User(ctx context.Context, id *ident.Ident, email *string) (*user.User, error) {
	if id != nil {
		return user.GetByIdent(ctx, *id)
	}

	if email != nil {
		return user.GetByEmail(ctx, *email)
	}

	return nil, apierror.Errorf("Either id or email must be specified")
}

func (r *userResolver) Teams(ctx context.Context, obj *user.User, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *team.TeamMembershipOrder) (*pagination.Connection[*team.TeamMember], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return team.ListForUser(ctx, obj.UUID, page, orderBy)
}

func (r *userResolver) IsAdmin(ctx context.Context, obj *user.User) (bool, error) {
	panic(fmt.Errorf("not implemented: IsAdmin - isAdmin"))
}

func (r *Resolver) User() gengqlv1.UserResolver { return &userResolver{r} }

type userResolver struct{ *Resolver }
