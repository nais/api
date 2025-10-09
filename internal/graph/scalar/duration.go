package scalar

import (
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
)

func UnmarshalDuration(v any) (time.Duration, error) {
	input, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("input must be a string")
	}

	d, err := time.ParseDuration(input)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format: %w", err)
	}

	return d, nil
}

func MarshalDuration(d time.Duration) graphql.Marshaler {
	return graphql.MarshalString(d.String())
}
