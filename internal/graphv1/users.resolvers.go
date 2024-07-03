package graphv1

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/graphv1/scalar"
	"github.com/nais/api/internal/users"
)

func (r *queryResolver) Users(ctx context.Context, first *int, after *scalar.Cursor, last *int, before *scalar.Cursor, orderBy *users.UserOrder) (*pagination.Connection[*users.User], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return users.List(ctx, page, orderBy)
}

func (r *queryResolver) User(ctx context.Context, id *string, email *string) (*users.User, error) {
	if id != nil {
		uid, err := uuid.Parse(*id)
		if err != nil {
			return nil, err
		}
		return users.Get(ctx, uid)
	}

	if email != nil {
		return users.GetByEmail(ctx, *email)
	}

	return nil, apierror.Errorf("Either id or email must be specified")
}
