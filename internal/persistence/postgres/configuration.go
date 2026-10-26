package postgres

import (
	"fmt"
	"maps"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type PostgresConfiguration struct {
	Parameters []*PostgresParameter `json:"parameters"`
}

type PostgresParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Read the additive status field directly until it is available in the published
// pgrator API types. This also supports branches reconciled by older pgrators.
func toPostgresConfiguration(branch *unstructured.Unstructured) (*PostgresConfiguration, error) {
	value, found, err := unstructured.NestedFieldNoCopy(branch.Object, "status", "configuration")
	if err != nil {
		return nil, fmt.Errorf("reading PostgresBranch %q configuration: %w", branch.GetName(), err)
	}
	if !found || value == nil {
		return nil, nil
	}
	configuration, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("PostgresBranch %q configuration has unexpected type %T", branch.GetName(), value)
	}
	values, _, err := unstructured.NestedStringMap(configuration, "parameters")
	if err != nil {
		return nil, fmt.Errorf("reading PostgresBranch %q parameters: %w", branch.GetName(), err)
	}
	parameters := make([]*PostgresParameter, 0, len(values))
	for _, name := range slices.Sorted(maps.Keys(values)) {
		parameters = append(parameters, &PostgresParameter{Name: name, Value: values[name]})
	}
	return &PostgresConfiguration{Parameters: parameters}, nil
}
