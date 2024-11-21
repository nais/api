package unleash

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identUnleashInstance identType = iota
)

func init() {
	ident.RegisterIdentType(identUnleashInstance, "UN", GetByIdent)
}

func newUnleashIdent(slug slug.Slug, name string) ident.Ident {
	return ident.NewIdent(identUnleashInstance, slug.String(), name)
}

func parseUnleashInstanceIdent(id ident.Ident) (slg slug.Slug, name string, err error) {
	parts := id.Parts()
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid unleash instance ident")
	}

	return slug.Slug(parts[0]), parts[1], nil
}
