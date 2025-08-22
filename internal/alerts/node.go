package alerts

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
	ident.RegisterIdentType(identKey, "ALRT", GetByIdent)
}

func newIdent(teamSlug slug.Slug, environment, alertName string) ident.Ident {
	return ident.NewIdent(identKey, teamSlug.String(), environment, alertName)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environment, alertName string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}
