package model

import (
	"strings"

	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
)

type Unleash struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	AllowedTeams []string       `json:"allowedTeams"`
	WebIngress   string         `json:"webIngress"`
	APIIngress   string         `json:"apiIngress"`
	Metrics      UnleashMetrics `json:"metrics"`
}

type UnleashMetrics struct {
	NumToggles int     `json:"numToggles"`
	APITokens  int     `json:"apiTokens"`
	Users      int     `json:"users"`
	CpuUsage   float64 `json:"cpuUsage"`
}

func ToUnleashInstance(u *unleash_nais_io_v1.Unleash) *Unleash {
	teams := []string{}
	for _, env := range u.Spec.ExtraEnvVars {
		if env.Name == "TEAMS_ALLOWED_TEAMS" {
			teams = strings.Split(env.Value, ",")
		}
	}

	return &Unleash{
		Name:         u.Name,
		Version:      u.Status.Version,
		AllowedTeams: teams,
		WebIngress:   u.Spec.WebIngress.Host,
		APIIngress:   u.Spec.ApiIngress.Host,
	}
}
