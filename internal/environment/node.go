package environment

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identEnvironment identType = iota
)

func init() {
	ident.RegisterIdentType(identEnvironment, "E", getByIdent)
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

func getByIdent(ctx context.Context, id ident.Ident) (*Environment, error) {
	name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	e, err := db(ctx).Get(ctx, name)
	if err != nil {
		return nil, err
	}

	return toGraphEnvironment(e), nil
}
