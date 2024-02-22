package model

import "github.com/nais/api/internal/graph/scalar"

type User struct {
	ID         scalar.Ident `json:"id"`
	Email      string       `json:"email"`
	Name       string       `json:"name"`
	ExternalID string       `json:"externalId"`
}

func (User) IsAuthenticatedUser() {}
