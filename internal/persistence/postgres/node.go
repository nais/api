package postgres

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identZalandoPostgres identType = iota
)

func init() {
	ident.RegisterIdentType(identZalandoPostgres, "PP", GetZalandoPostgresByIdent)
}

func newZalandoPostgresIdent(teamSlug slug.Slug, environmentName, postgresInstanceName string) ident.Ident {
	return ident.NewIdent(identZalandoPostgres, teamSlug.String(), environmentName, postgresInstanceName)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environmentName, postgresInstanceName string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}

func newIdent(teamSlug slug.Slug, environmentName, postgresInstanceName string) ident.Ident {
	return ident.NewIdent(identZalandoPostgres, teamSlug.String(), environmentName, postgresInstanceName)
}
