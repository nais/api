package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/role/graphrole"
)

func (r *queryResolver) Roles(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*graphrole.Role], error) {
	panic(fmt.Errorf("not implemented: Roles - roles"))
}
