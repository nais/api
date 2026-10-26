package environment

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/crd"
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

type AvailableValueOptions struct {
	EnvironmentName string
	Name            string
	Values          []string
	DefaultValues   []string
}

func (AvailableValueOptions) IsNode() {}

func (o AvailableValueOptions) ID() ident.Ident {
	return ident.NewIdent(identAvailableValueOptions, o.EnvironmentName, o.Name)
}

func GetAvailableValueOptions(ctx context.Context, environmentName, name string) (*AvailableValueOptions, error) {
	options, err := crd.Options(ctx, environmentName, name)
	if err != nil {
		return nil, err
	}
	return &AvailableValueOptions{
		EnvironmentName: environmentName,
		Name:            name,
		Values:          options.Values,
		DefaultValues:   options.DefaultValues,
	}, nil
}

func getAvailableValueOptionsByIdent(ctx context.Context, id ident.Ident) (*AvailableValueOptions, error) {
	parts := id.Parts()
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid string options ident")
	}
	return GetAvailableValueOptions(ctx, parts[0], parts[1])
}
