package job

import (
	"fmt"

	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "J", ident.Wrap(GetByIdent))
}

func newIdent(teamSlug slug.Slug, environment, name string) ident.Ident {
	return ident.NewIdent(identKey, teamSlug.String(), environment, name)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environment, name string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid job ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}
