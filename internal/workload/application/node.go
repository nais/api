package application

import (
	"fmt"

	ident2 "github.com/nais/api/internal/graphv1/ident"

	"github.com/nais/api/internal/slug"
)

type identType int

const (
	ident identType = iota
)

func init() {
	ident2.RegisterIdentType(ident, "A", ident2.Wrap(GetByIdent))
}

func newIdent(teamSlug slug.Slug, environment, name string) ident2.Ident {
	return ident2.NewIdent(ident, teamSlug.String(), environment, name)
}

func parseIdent(id ident2.Ident) (teamSlug slug.Slug, environment, name string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid application ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}
