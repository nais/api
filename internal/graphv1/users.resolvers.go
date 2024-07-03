package graphv1

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/graphv1/scalar"
	"github.com/nais/api/internal/user"
)

func (r *queryResolver) Users(ctx context.Context, first *int, after *scalar.Cursor, last *int, before *scalar.Cursor, orderBy *user.UserOrder) (*pagination.Connection[*user.User], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return user.List(ctx, page, orderBy)
}

func (r *queryResolver) User(ctx context.Context, id *string, email *string) (*user.User, error) {
	if id != nil {
		uid, err := uuid.Parse(*id)
		if err != nil {
			return nil, err
		}
		return user.Get(ctx, uid)
	}

	if email != nil {
		return user.GetByEmail(ctx, *email)
	}

	return nil, apierror.Errorf("Either id or email must be specified")
}
