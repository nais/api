package graphv1

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/graphv1/scalar"
)

func (r *queryResolver) Users(ctx context.Context, first *int, after *scalar.Cursor, last *int, before *scalar.Cursor, orderBy *modelv1.UserOrder) (*pagination.Connection[*modelv1.User], error) {
	panic(fmt.Errorf("not implemented: Users - users"))
}

func (r *queryResolver) User(ctx context.Context, id *uuid.UUID, email *string) (*modelv1.User, error) {
	panic(fmt.Errorf("not implemented: User - user"))
}
