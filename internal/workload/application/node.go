package application

import (
	"fmt"

	"github.com/nais/api/internal/graphv1/scalar"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	ident identType = iota
)

func init() {
	scalar.RegisterIdentType(ident, "A", scalar.Wrap(GetByIdent))
}

func newIdent(teamSlug slug.Slug, environment, name string) scalar.Ident {
	return scalar.NewIdent(ident, teamSlug.String(), environment, name)
}

func parseIdent(id scalar.Ident) (teamSlug slug.Slug, environment, name string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid application ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}
