package reconciler

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	reconcilerIdentKey identType = iota
	reconcilerErrorIdentKey
)

func init() {
	ident.RegisterIdentType(reconcilerIdentKey, "REC", GetByIdent)
	ident.RegisterIdentType(reconcilerErrorIdentKey, "RECE", getReconcilerErrorByIdent)
}

func newReconcilerIdent(name string) ident.Ident {
	return ident.NewIdent(reconcilerIdentKey, name)
}

func newReconcilerErrorIdent(id uuid.UUID) ident.Ident {
	return ident.NewIdent(reconcilerErrorIdentKey, base58.Encode(id[:]))
}

func parseReconcilerIdent(id ident.Ident) (string, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid reconciler ident")
	}

	return parts[0], nil
}

func parseReconcilerErrorIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid reconciler error ident")
	}

	return uuid.FromBytes(base58.Decode(parts[0]))
}

func getReconcilerErrorByIdent(ctx context.Context, ident ident.Ident) (*ReconcilerError, error) {
	id, err := parseReconcilerErrorIdent(ident)
	if err != nil {
		return nil, err
	}

	e, err := db(ctx).GetReconcilerErrorByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return toGraphReconcilerError(e), nil
}
