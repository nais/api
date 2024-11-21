package repository

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "GR", getByIdent)
}

func newIdent(teamSlug slug.Slug, name string) ident.Ident {
	return ident.NewIdent(identKey, teamSlug.String(), name)
}

func parseIdent(id ident.Ident) (slug.Slug, string, error) {
	parts := id.Parts()
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repository ident")
	}

	return slug.Slug(parts[0]), parts[1], nil
}
