package environment

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/ident"
)

type AvailableValues struct {
	EnvironmentName string
}

func (AvailableValues) IsNode() {}

func (v AvailableValues) ID() ident.Ident {
	return ident.NewIdent(identAvailableValues, v.EnvironmentName)
}

func getAvailableValuesByIdent(ctx context.Context, id ident.Ident) (*AvailableValues, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return nil, fmt.Errorf("invalid available values ident")
	}
	env, err := Get(ctx, parts[0])
	if err != nil {
		return nil, err
	}
	return &AvailableValues{EnvironmentName: env.Name}, nil
}
