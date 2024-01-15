package model

import "github.com/nais/api/internal/graph/scalar"

type ServiceAccount struct {
	// Unique ID of the service account.
	ID scalar.Ident `json:"id"`
	// The name of the service account.
	Name string `json:"name"`
}

func (ServiceAccount) IsAuthenticatedUser() {}
