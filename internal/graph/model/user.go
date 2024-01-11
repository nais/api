package model

import "github.com/nais/api/internal/graph/scalar"

// User type.
type User struct {
	// Unique ID of the user.
	ID scalar.Ident `json:"id"`
	// The email address of the user.
	Email string `json:"email"`
	// The name of the user.
	Name string `json:"name"`
	// The external ID of the user.
	ExternalID string `json:"externalId"`
}

func (User) IsAuthenticatedUser() {}
