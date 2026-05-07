package serviceaccount

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identServiceAccount identType = iota
	identServiceAccountToken
	identServiceAccountWorkloadBinding
)

func init() {
	ident.RegisterIdentType(identServiceAccount, "SA", GetByIdent)
	ident.RegisterIdentType(identServiceAccountToken, "SAT", GetTokenByIdent)
	ident.RegisterIdentType(identServiceAccountWorkloadBinding, "SAB", GetBindingByIdent)
}

func newIdent(uid uuid.UUID) ident.Ident {
	return ident.NewIdent(identServiceAccount, base58.Encode(uid[:]))
}

func newTokenIdent(uid uuid.UUID) ident.Ident {
	return ident.NewIdent(identServiceAccountToken, base58.Encode(uid[:]))
}

func newBindingIdent(uid uuid.UUID) ident.Ident {
	return ident.NewIdent(identServiceAccountWorkloadBinding, base58.Encode(uid[:]))
}

func parseIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, apierror.Errorf("The provided service account ID is not valid.")
	}

	uid, err := uuid.FromBytes(base58.Decode(parts[0]))
	if err != nil {
		return uuid.Nil, apierror.Errorf("The provided service account ID is not valid.")
	}
	return uid, nil
}

func parseTokenIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, apierror.Errorf("The provided service account token ID is not valid.")
	}

	uid, err := uuid.FromBytes(base58.Decode(parts[0]))
	if err != nil {
		return uuid.Nil, apierror.Errorf("The provided service account token ID is not valid.")
	}
	return uid, nil
}

func parseBindingIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, apierror.Errorf("The provided service account workload binding ID is not valid.")
	}

	uid, err := uuid.FromBytes(base58.Decode(parts[0]))
	if err != nil {
		return uuid.Nil, apierror.Errorf("The provided service account workload binding ID is not valid.")
	}
	return uid, nil
}
