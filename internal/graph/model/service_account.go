package model

import (
	"github.com/google/uuid"
)

type ServiceAccount struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (ServiceAccount) IsAuthenticatedUser() {}
