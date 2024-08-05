package sqlinstance

import (
	"fmt"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
)

type identType int

const (
	identSqlInstance identType = iota
	identSqlDatabase
)

func init() {
	ident.RegisterIdentType(identSqlInstance, "PSI", ident.Wrap(GetByIdent))
	ident.RegisterIdentType(identSqlDatabase, "PSD", ident.Wrap(GetDatabaseByIdent))
}

func newIdent(teamSlug slug.Slug, environmentName, sqlInstanceName string) ident.Ident {
	return ident.NewIdent(identSqlInstance, teamSlug.String(), environmentName, sqlInstanceName)
}

func newDatabaseIdent(teamSlug slug.Slug, environmentName, sqlInstanceName string) ident.Ident {
	return ident.NewIdent(identSqlDatabase, teamSlug.String(), environmentName, sqlInstanceName)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environmentName, sqlInstanceName string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}
