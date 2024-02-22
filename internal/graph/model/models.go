package model

import (
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type TeamMember struct {
	TeamRole TeamRole
	TeamSlug slug.Slug
	UserID   scalar.Ident
}
