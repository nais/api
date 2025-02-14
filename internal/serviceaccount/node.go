package serviceaccount

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identServiceAccount identType = iota
	identServiceAccountToken
)

func init() {
	ident.RegisterIdentType(identServiceAccount, "SA", GetByIdent)
	ident.RegisterIdentType(identServiceAccountToken, "SAT", GetTokenByIdent)
}

func newIdent(uid uuid.UUID) ident.Ident {
	return ident.NewIdent(identServiceAccount, base58.Encode(uid[:]))
}

func newTokenIdent(uid uuid.UUID) ident.Ident {
	return ident.NewIdent(identServiceAccountToken, base58.Encode(uid[:]))
}

func parseIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid service account ident")
	}

	return uuid.FromBytes(base58.Decode(parts[0]))
}

func parseTokenIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid service account token ident")
	}

	return uuid.FromBytes(base58.Decode(parts[0]))
}
