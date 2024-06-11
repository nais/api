package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type Reconciler struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	MemberAware bool   `json:"memberAware"`
}

type ReconcilerError struct {
	ID            scalar.Ident `json:"id"`
	CorrelationID uuid.UUID    `json:"correlationId"`
	CreatedAt     time.Time    `json:"createdAt"`
	Message       string       `json:"message"`
	TeamSlug      slug.Slug    `json:"teamSlug"`
}
