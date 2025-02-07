package authz

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identRole identType = iota
)

func init() {
	ident.RegisterIdentType(identRole, "ROL", getRoleByIdent)
}

func newRoleIdent(name string) ident.Ident {
	return ident.NewIdent(identRole, name)
}

func parseRoleIdent(id ident.Ident) (string, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid role ident")
	}

	return parts[0], nil
}
