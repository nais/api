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
	// Slug of the team to add the repository to.
	TeamSlug slug.Slug `json:"teamSlug"`
	// Name of the repository, with the org prefix, for instance 'org/repo'.
	RepoName string `json:"repoName"`
}

type AddRepositoryToTeamPayload struct {
	// Repository that was added to the team.
	Repository *Repository `json:"repository"`
}

type RemoveRepositoryFromTeamInput struct {
	// Slug of the team to remove the repository from.
	TeamSlug slug.Slug `json:"teamSlug"`
	// Name of the repository, with the org prefix, for instance 'org/repo'.
	RepoName string `json:"repoName"`
}

type RemoveRepositoryFromTeamPayload struct {
	// Repository that was removed from the team.
	Repository *Repository `json:"repository"`
}
