package application

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identKey identType = iota
	identInstanceKey
)

func init() {
	ident.RegisterIdentType(identKey, "A", GetByIdent)
	ident.RegisterIdentType(identInstanceKey, "INS", getInstanceByIdent)
}

func newIdent(teamSlug slug.Slug, environment, name string) ident.Ident {
	return ident.NewIdent(identKey, teamSlug.String(), environment, name)
}

func newInstanceIdent(teamSlug slug.Slug, environment, applicationName, instanceName string) ident.Ident {
	return ident.NewIdent(identInstanceKey, teamSlug.String(), environment, applicationName, instanceName)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environment, name string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid application ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}

func parseInstanceIdent(id ident.Ident) (teamSlug slug.Slug, environment, applicationName, instanceName string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", "", fmt.Errorf("invalid application ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], parts[3], nil
}
