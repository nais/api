package modelv1

import "github.com/nais/api/internal/graphv1/pagination"

type (
	TeamConnection = pagination.Connection[*Team]
	TeamEdge       = pagination.Edge[*Team]
)
