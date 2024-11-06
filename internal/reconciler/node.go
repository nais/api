package reconciler

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "REC", GetByIdent)
}

func newIdent(name string) ident.Ident {
	return ident.NewIdent(identKey, name)
}

func parseIdent(id ident.Ident) (string, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid reconciler ident")
	}

	return parts[0], nil
}
