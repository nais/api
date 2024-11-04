package deployment

import (
	"fmt"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
)

type identType int

const (
	identKeyDeploymentKey identType = iota
	identKeyDeployment
)

func init() {
	ident.RegisterIdentType(identKeyDeploymentKey, "DK", getDeploymentKeyByIdent)
	ident.RegisterIdentType(identKeyDeployment, "DI", getDeploymentByIdent)
}

func newDeploymentKeyIdent(slug slug.Slug) ident.Ident {
	return ident.NewIdent(identKeyDeploymentKey, slug.String())
}

func parseDeploymentKeyIdent(id ident.Ident) (slug.Slug, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid deployment key ident")
	}

	return slug.Slug(parts[0]), nil
}

func newDeploymentIdent(id string) ident.Ident {
	return ident.NewIdent(identKeyDeployment, id)
}

func parseDeploymentIdent(id ident.Ident) (string, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid deployment ident")
	}

	return parts[0], nil
}
