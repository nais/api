package model

import (
	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

type Role struct {
	Name     string      `json:"name"`
	IsGlobal bool        `json:"isGlobal"`
	GQLVars  RoleGQLVars `json:"-"`
}

type RoleGQLVars struct {
	TargetServiceAccountID uuid.UUID
	TargetTeamSlug         *slug.Slug
}
