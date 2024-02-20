package model

import (
	"github.com/google/uuid"
)

type User struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	ExternalID string    `json:"externalId"`
}

func (User) IsAuthenticatedUser() {}
