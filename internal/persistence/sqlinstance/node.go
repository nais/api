package sqlinstance

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identSQLInstance identType = iota
	identSQLDatabase
)

func init() {
	ident.RegisterIdentType(identSQLInstance, "PSI", GetByIdent)
	ident.RegisterIdentType(identSQLDatabase, "PSD", GetDatabaseByIdent)
}

func newIdent(teamSlug slug.Slug, environmentName, sqlInstanceName string) ident.Ident {
	return ident.NewIdent(identSQLInstance, teamSlug.String(), environmentName, sqlInstanceName)
}

func newDatabaseIdent(teamSlug slug.Slug, environmentName, sqlInstanceName string) ident.Ident {
	return ident.NewIdent(identSQLDatabase, teamSlug.String(), environmentName, sqlInstanceName)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environmentName, sqlInstanceName string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}
