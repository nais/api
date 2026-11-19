package postgres

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identPostgresAccess identType = iota
	identPostgres
)

func init() {
	ident.RegisterIdentType(identPostgresAccess, "PA", GetPostgresAccessByIdent)
	ident.RegisterIdentType(identPostgres, "PG", GetPostgresByIdent)
}

func parseAccessIdent(id ident.Ident) (teamSlug slug.Slug, environmentName, name string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}

func newAccessIdent(teamSlug slug.Slug, environmentName, name string) ident.Ident {
	return ident.NewIdent(identPostgresAccess, teamSlug.String(), environmentName, name)
}

func newPostgresIdent(teamSlug slug.Slug, environmentName, name string) ident.Ident {
	return ident.NewIdent(identPostgres, teamSlug.String(), environmentName, name)
}
