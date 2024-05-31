package model

import (
	"strings"

	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
)

type Unleash struct {
	Instance *UnleashInstance `json:"instance"`
	Enabled  bool             `json:"enabled"`
}

type UnleashInstance struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	AllowedTeams []string       `json:"allowedTeams"`
	WebIngress   string         `json:"webIngress"`
	APIIngress   string         `json:"apiIngress"`
	Metrics      UnleashMetrics `json:"metrics"`
	Ready        bool           `json:"ready"`
}

type UnleashMetrics struct {
	CpuRequests    float64        `json:"cpuRequests"`
	MemoryRequests float64        `json:"memoryRequests"`
	GQLVars        UnleashGQLVars `json:"-"` // Internal context for custom resolvers
}

type UnleashGQLVars struct {
	Namespace    string
	InstanceName string
}

func ToUnleashInstance(u *unleash_nais_io_v1.Unleash) *UnleashInstance {
	var teams []string
	for _, env := range u.Spec.ExtraEnvVars {
		if env.Name == "TEAMS_ALLOWED_TEAMS" {
			teams = strings.Split(env.Value, ",")
		}
	}

	return &UnleashInstance{
		Name:         u.Name,
		Version:      u.Status.Version,
		AllowedTeams: teams,
		WebIngress:   u.Spec.WebIngress.Host,
		APIIngress:   u.Spec.ApiIngress.Host,
		Ready:        u.Status.Reconciled && u.Status.Connected,
		Metrics: UnleashMetrics{
			CpuRequests:    u.Spec.Resources.Requests.Cpu().AsApproximateFloat64(),
			MemoryRequests: u.Spec.Resources.Requests.Memory().AsApproximateFloat64(),
			GQLVars: UnleashGQLVars{
				Namespace:    u.Namespace,
				InstanceName: u.Name,
			},
		},
	}
}
