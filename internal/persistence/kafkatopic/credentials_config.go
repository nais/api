package kafkatopic

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/apierror"
)

// EnvironmentConfig configures the Kafka pool and aliases for a Nais environment.
type EnvironmentConfig struct {
	Pool    string   `json:"pool"`
	Aliases []string `json:"aliases,omitempty"`
}

// Environments is keyed by canonical, user-facing Nais environment names.
type Environments map[string]EnvironmentConfig

func (e *Environments) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	type environments Environments
	var decoded environments
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("unmarshalling Kafka environments: %w", err)
	}
	*e = Environments(decoded)
	return nil
}

// Validate rejects incomplete mappings, unknown clusters and ambiguous aliases.
func (e Environments) Validate(clusters []string) error {
	aliases := make(map[string]string)
	for name, target := range e {
		if name == "" || target.Pool == "" {
			return fmt.Errorf("kafka environment %q requires a name and pool", name)
		}
		cluster := environmentmapper.ClusterName(name)
		if !slices.Contains(clusters, cluster) || environmentmapper.EnvironmentName(cluster) != name {
			return fmt.Errorf("kafka environment %q is not a configured Nais environment", name)
		}
		for _, alias := range target.Aliases {
			if alias == "" {
				return fmt.Errorf("kafka environment %q has an empty alias", name)
			}
			if _, ok := e[alias]; ok {
				return fmt.Errorf("kafka environment alias %q conflicts with a canonical environment", alias)
			}
			if previous, ok := aliases[alias]; ok {
				return fmt.Errorf("kafka environment alias %q is configured for both %q and %q", alias, previous, name)
			}
			if environmentmapper.EnvironmentName(environmentmapper.ClusterName(alias)) == alias && slices.Contains(clusters, environmentmapper.ClusterName(alias)) {
				return fmt.Errorf("kafka environment alias %q conflicts with a Nais environment", alias)
			}
			aliases[alias] = name
		}
	}
	return nil
}

func (e Environments) resolve(input string) (string, EnvironmentConfig, error) {
	if target, ok := e[input]; ok {
		return input, target, nil
	}
	for name, target := range e {
		if slices.Contains(target.Aliases, input) {
			return name, target, nil
		}
	}

	valid := make([]string, 0, len(e))
	for name := range e {
		valid = append(valid, name)
	}
	slices.Sort(valid)
	if len(valid) == 0 {
		return "", EnvironmentConfig{}, apierror.Errorf("Kafka credentials are not configured for any Nais environment")
	}
	return "", EnvironmentConfig{}, apierror.Errorf("Kafka credentials are not configured for Nais environment %q; valid environments: %s", input, strings.Join(valid, ", "))
}
