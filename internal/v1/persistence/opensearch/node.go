package opensearch

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
	ident.RegisterIdentType(identKey, "POS", GetByIdent)
}

func newIdent(teamSlug slug.Slug, environment, name string) ident.Ident {
	return ident.NewIdent(identKey, teamSlug.String(), environment, name)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environment, name string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}