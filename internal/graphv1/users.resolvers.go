package graphv1

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graphv1/modelv1"
)

func (r *queryResolver) Users(ctx context.Context, first *int, after *string, last *int, before *string, orderBy *modelv1.UserOrder) (*modelv1.UserConnection, error) {
	panic(fmt.Errorf("not implemented: Users - users"))
}

func (r *queryResolver) User(ctx context.Context, id *uuid.UUID, email *string) (*modelv1.User, error) {
	panic(fmt.Errorf("not implemented: User - user"))
}
