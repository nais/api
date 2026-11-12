package kafkatopic

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/apierror"
)

// CredentialEnvironment configures the Kafka pool and aliases for a Nais environment.
type CredentialEnvironment struct {
	Pool    string   `json:"pool"`
	Aliases []string `json:"aliases,omitempty"`
}

// CredentialEnvironments is keyed by canonical, user-facing Nais environment names.
type CredentialEnvironments map[string]CredentialEnvironment

func (e *CredentialEnvironments) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	type environments CredentialEnvironments
	var decoded environments
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("unmarshalling Kafka credential environments: %w", err)
	}
	*e = CredentialEnvironments(decoded)
	return nil
}

// Validate rejects incomplete mappings, unknown clusters and ambiguous aliases.
func (e CredentialEnvironments) Validate(clusters []string) error {
	aliases := make(map[string]string)
	for name, target := range e {
		if name == "" || target.Pool == "" {
			return fmt.Errorf("kafka credential environment %q requires a name and pool", name)
		}
		cluster := environmentmapper.ClusterName(name)
		if !slices.Contains(clusters, cluster) || environmentmapper.EnvironmentName(cluster) != name {
			return fmt.Errorf("kafka credential environment %q is not a configured Nais environment", name)
		}
		for _, alias := range target.Aliases {
			if alias == "" {
				return fmt.Errorf("kafka credential environment %q has an empty alias", name)
			}
			if _, ok := e[alias]; ok {
				return fmt.Errorf("kafka credential alias %q conflicts with a canonical environment", alias)
			}
			if previous, ok := aliases[alias]; ok {
				return fmt.Errorf("kafka credential alias %q is configured for both %q and %q", alias, previous, name)
			}
			if environmentmapper.EnvironmentName(environmentmapper.ClusterName(alias)) == alias && slices.Contains(clusters, environmentmapper.ClusterName(alias)) {
				return fmt.Errorf("kafka credential alias %q conflicts with a Nais environment", alias)
			}
			aliases[alias] = name
		}
	}
	return nil
}

func (e CredentialEnvironments) resolve(input string) (string, CredentialEnvironment, error) {
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
		return "", CredentialEnvironment{}, apierror.Errorf("Kafka credentials are not configured for any Nais environment")
	}
	return "", CredentialEnvironment{}, apierror.Errorf("Kafka credentials are not configured for Nais environment %q; valid environments: %s", input, strings.Join(valid, ", "))
}
