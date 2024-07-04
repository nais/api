package team

import (
	"fmt"
	"github.com/nais/api/internal/graphv1/ident"

	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identTeam identType = iota
	identTeamEnvironment
)

func init() {
	ident.RegisterIdentType(identTeam, "T", ident.Wrap(GetByIdent))
	ident.RegisterIdentType(identTeamEnvironment, "TE", ident.Wrap(GetTeamEnvironmentByIdent))
}

func newTeamIdent(slug slug.Slug) ident.Ident {
	return ident.NewIdent(identTeam, slug.String())
}

func newTeamEnvironmentIdent(slug slug.Slug, envName string) ident.Ident {
	return ident.NewIdent(identTeamEnvironment, slug.String(), envName)
}

func parseTeamIdent(id ident.Ident) (slug.Slug, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid team ident")
	}

	return slug.Slug(parts[0]), nil
}

func parseTeamEnvironmentIdent(id ident.Ident) (slug.Slug, string, error) {
	parts := id.Parts()
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid team environment ident")
	}

	return slug.Slug(parts[0]), parts[1], nil
}
