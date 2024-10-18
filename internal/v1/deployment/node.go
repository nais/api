package deployment

import (
	"fmt"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "DK", getByIdent)
}

func newIdent(slug slug.Slug) ident.Ident {
	return ident.NewIdent(identKey, slug.String())
}

func parseIdent(id ident.Ident) (slug.Slug, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid team ident")
	}

	return slug.Slug(parts[0]), nil
}
