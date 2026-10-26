package environment

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identEnvironment identType = iota
	identAvailableValues
	identAvailableValueOptions
)

func init() {
	ident.RegisterIdentType(identEnvironment, "E", GetByIdent)
	ident.RegisterIdentType(identAvailableValues, "AV", getAvailableValuesByIdent)
	ident.RegisterIdentType(identAvailableValueOptions, "SO", getAvailableValueOptionsByIdent)
}

func newIdent(name string) ident.Ident {
	return ident.NewIdent(identEnvironment, name)
}

func parseIdent(id ident.Ident) (string, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid environment ident")
	}

	return parts[0], nil
}
