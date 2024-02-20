package model

import (
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type GitHubRepository struct {
	ID          scalar.Ident                  `json:"id"`
	Name        string                        `json:"name"`
	Permissions []*GitHubRepositoryPermission `json:"permissions"`
	RoleName    string                        `json:"roleName"`
	Archived    bool                          `json:"archived"`
	GQLVars     GitHubRepositoryGQLVars       `json:"-"`
}

type GitHubRepositoryGQLVars struct {
	TeamSlug slug.Slug
}
