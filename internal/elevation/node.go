package elevation

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const identKey identType = iota

func init() {
	ident.RegisterIdentType(identKey, "ELEV", GetByIdent)
}

func newIdent(teamSlug slug.Slug, environmentName, elevationID string) ident.Ident {
	return ident.NewIdent(identKey, teamSlug.String(), environmentName, elevationID)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environmentName, elevationID string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid elevation ident")
	}
	return slug.Slug(parts[0]), parts[1], parts[2], nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Elevation, error) {
	teamSlug, environmentName, elevationID, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, environmentName, elevationID)
}
