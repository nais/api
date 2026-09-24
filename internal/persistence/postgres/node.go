package postgres

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identPostgres identType = iota
	identPostgresAccess
)

func init() {
	ident.RegisterIdentType(identPostgres, "PP", GetPostgresByIdent)
	ident.RegisterIdentType(identPostgresAccess, "PA", GetPostgresAccessByIdent)
}

func parsePostgresInstanceIdent(id ident.Ident) (teamSlug slug.Slug, environmentName, postgresInstanceName string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}

func newIdent(teamSlug slug.Slug, environmentName, postgresInstanceName string) ident.Ident {
	return ident.NewIdent(identPostgres, teamSlug.String(), environmentName, postgresInstanceName)
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
