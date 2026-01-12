package elevation

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
)

type identType int

const identKey identType = iota

func init() {
	ident.RegisterIdentType(identKey, "ELEV", GetByIdent)
}

func newIdent(elevationID string) ident.Ident {
	return ident.NewIdent(identKey, elevationID)
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Elevation, error) {
	// This is not used since we don't implement Node interface,
	// but is required for ident registration
	return nil, nil
}
