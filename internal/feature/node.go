package feature

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "F", getByIdent)
}

func NewIdent(feature string) ident.Ident {
	return ident.NewIdent(identKey, feature)
}

func parseIdent(id ident.Ident) (feature string, err error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid feature ident")
	}

	return parts[0], nil
}
