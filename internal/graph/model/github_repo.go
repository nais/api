package model

import (
	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

// GitHub repository type.
type GitHubRepository struct {
	// ID of the repository.
	ID uuid.UUID `json:"id"`
	// Name of the repository, with the org prefix.
	Name string `json:"name"`
	// A list of permissions given to the team for this repository.
	Permissions []*GitHubRepositoryPermission `json:"permissions"`
	// The name of the role the team has been granted in the repository.
	RoleName string `json:"roleName"`
	// Whether or not the repository is archived.
	Archived bool `json:"archived"`

	GQLVars GitHubRepositoryGQLVars `json:"-"`
}

type GitHubRepositoryGQLVars struct {
	TeamSlug slug.Slug
}
