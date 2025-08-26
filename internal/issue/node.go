package issue

import (
	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "I", GetByIdent)
}

func newIdent(id string) ident.Ident {
	return ident.NewIdent(identKey, id)
}
