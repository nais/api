package user

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graphv1/scalar"
)

type identType int

const (
	ident identType = iota
)

func init() {
	scalar.RegisterIdentType(ident, "U", scalar.Wrap(GetByIdent))
}

func newIdent(uid uuid.UUID) scalar.Ident {
	return scalar.NewIdent(ident, uid.String())
}

func parseIdent(id scalar.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid user ident")
	}

	return uuid.Parse(parts[0])
}
