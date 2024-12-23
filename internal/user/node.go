package user

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "U", GetByIdent)
}

func NewIdent(uid uuid.UUID) ident.Ident {
	return ident.NewIdent(identKey, base58.Encode(uid[:]))
}

func parseIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid user ident")
	}

	return uuid.FromBytes(base58.Decode(parts[0]))
}
