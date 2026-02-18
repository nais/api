package repository

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/nais/api/internal/github/repository/repositorysql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
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

type RepositoryOrder struct {
	Field     RepositoryOrderField `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

func (o *RepositoryOrder) String() string {
	if o == nil {
		return ""
	}

	return strings.ToLower(o.Field.String() + ":" + o.Direction.String())
}

type RepositoryOrderField string

const (
	RepositoryOrderFieldName RepositoryOrderField = "NAME"
)

var AllRepositoryOrderField = []RepositoryOrderField{
	RepositoryOrderFieldName,
}

func (e RepositoryOrderField) IsValid() bool {
	return slices.Contains(AllRepositoryOrderField, e)
}

func (e RepositoryOrderField) String() string {
	return string(e)
}

func (e *RepositoryOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = RepositoryOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid RepositoryOrderField", str)
	}
	return nil
}

func (e RepositoryOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
