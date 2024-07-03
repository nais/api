package users

import (
	"github.com/google/uuid"
	"github.com/nais/api/internal/graphv1/pagination"
)

type (
	UserConnection = pagination.Connection[*User]
	UserEdge       = pagination.Edge[*User]
)

type User struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	ExternalID string    `json:"externalId"`
	IsAdmin    bool      `json:"isAdmin"`
}
