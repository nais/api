package users

import (
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
)

type (
	UserConnection = pagination.Connection[*modelv1.User]
	UserEdge       = pagination.Edge[*modelv1.User]
)
