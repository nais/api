package repository

import (
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/github/repository/repositorysql"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

type (
	RepositoryConnection = pagination.Connection[*Repository]
	RepositoryEdge       = pagination.Edge[*Repository]
)

type Repository struct {
	Name     string    `json:"name"`
	TeamSlug slug.Slug `json:"-"`
}

func (Repository) IsNode() {}

func (r Repository) ID() ident.Ident {
	return newIdent(r.TeamSlug, r.Name)
}

func toGraphRepository(r *repositorysql.TeamRepository) *Repository {
	return &Repository{
		TeamSlug: r.TeamSlug,
		Name:     r.GithubRepository,
	}
}

type AddRepositoryToTeamInput struct {
	TeamSlug       slug.Slug `json:"teamSlug"`
	RepositoryName string    `json:"repositoryName"`
}

type AddRepositoryToTeamPayload struct {
	Repository *Repository `json:"repository"`
}

type RemoveRepositoryFromTeamInput struct {
	TeamSlug       slug.Slug `json:"teamSlug"`
	RepositoryName string    `json:"repositoryName"`
}

type RemoveRepositoryFromTeamPayload struct {
	Success bool `json:"success"`
}

type TeamRepositoryFilter struct {
	Name *string `json:"name"`
}
