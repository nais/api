package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/department"
	"github.com/nais/api/internal/graph/pagination"
)

func (r *mutationResolver) CreateDepartment(ctx context.Context, input department.CreateDepartmentInput) (*department.CreateDepartmentPayload, error) {
	panic(fmt.Errorf("not implemented: CreateDepartment - createDepartment"))
}

func (r *queryResolver) Departments(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*department.DepartmentConnection, error) {
	panic(fmt.Errorf("not implemented: Departments - departments"))
}
