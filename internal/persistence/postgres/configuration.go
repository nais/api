package postgres

import (
	"maps"
	"slices"

	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
)

type PostgresConfiguration struct {
	Parameters []*PostgresParameter `json:"parameters"`
}

type PostgresParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func toPostgresConfiguration(configuration *nais_io_v1.PostgresBranchConfiguration) *PostgresConfiguration {
	if configuration == nil {
		return nil
	}
	values := configuration.Parameters
	parameters := make([]*PostgresParameter, 0, len(values))
	for _, name := range slices.Sorted(maps.Keys(values)) {
		parameters = append(parameters, &PostgresParameter{Name: name, Value: values[name]})
	}
	return &PostgresConfiguration{Parameters: parameters}
}
