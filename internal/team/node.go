package team

import (
	"fmt"

	"github.com/nais/api/internal/graphv1/scalar"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identTeam identType = iota
	identTeamEnvironment
)

func init() {
	scalar.RegisterIdentType(identTeam, "T", scalar.Wrap(GetByIdent))
	scalar.RegisterIdentType(identTeamEnvironment, "TE", scalar.Wrap(GetTeamEnvironmentByIdent))
}

func newTeamIdent(slug slug.Slug) scalar.Ident {
	return scalar.NewIdent(identTeam, slug.String())
}

func newTeamEnvironmentIdent(slug slug.Slug, envName string) scalar.Ident {
	return scalar.NewIdent(identTeamEnvironment, slug.String(), envName)
}

func parseTeamIdent(id scalar.Ident) (slug.Slug, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid team ident")
	}

	return slug.Slug(parts[0]), nil
}

func parseTeamEnvironmentIdent(id scalar.Ident) (slug.Slug, string, error) {
	parts := id.Parts()
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid team environment ident")
	}

	return slug.Slug(parts[0]), parts[1], nil
}
