package model

import (
	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

type GitHubRepository struct {
	ID          uuid.UUID                     `json:"id"`
	Name        string                        `json:"name"`
	Permissions []*GitHubRepositoryPermission `json:"permissions"`
	RoleName    string                        `json:"roleName"`
	Archived    bool                          `json:"archived"`
	GQLVars     GitHubRepositoryGQLVars       `json:"-"`
}

type GitHubRepositoryGQLVars struct {
	TeamSlug slug.Slug
}
