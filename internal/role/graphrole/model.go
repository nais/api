package graphrole

import "github.com/nais/api/internal/graph/pagination"

type (
	RoleConnection = pagination.Connection[*Role]
	RoleEdge       = pagination.Edge[*Role]
)

type Role struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
