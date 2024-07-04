package user

import (
	"fmt"
	ident2 "github.com/nais/api/internal/graphv1/ident"

	"github.com/google/uuid"
)

type identType int

const (
	ident identType = iota
)

func init() {
	ident2.RegisterIdentType(ident, "U", ident2.Wrap(GetByIdent))
}

func newIdent(uid uuid.UUID) ident2.Ident {
	return ident2.NewIdent(ident, uid.String())
}

func parseIdent(id ident2.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid user ident")
	}

	return uuid.Parse(parts[0])
}
