package graphv1

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/graphv1/scalar"
	"github.com/nais/api/internal/users"
)

func (r *queryResolver) Users(ctx context.Context, first *int, after *scalar.Cursor, last *int, before *scalar.Cursor, orderBy *modelv1.UserOrder) (*pagination.Connection[*users.User], error) {
	panic(fmt.Errorf("not implemented: Users - users"))
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
