package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/pagination"
)

func (r *queryResolver) Roles(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*authz.Role], error) {
	panic(fmt.Errorf("not implemented: Roles - roles"))
}
