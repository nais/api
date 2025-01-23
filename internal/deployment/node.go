package deployment

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
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

func parseDeploymentIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid deployment ident")
	}

	return uuid.FromBytes(base58.Decode(parts[0]))
}

func newDeploymentIdent(id uuid.UUID) ident.Ident {
	return ident.NewIdent(identKeyDeployment, base58.Encode(id[:]))
}
