package config

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func ComputeFacets(ctx context.Context, allConfigs []*Config, filter *ConfigFilter) *ConfigFacets {
	filtered := SortFilter.Filter(ctx, allConfigs, filter)

	environmentCounts := map[string]int{}
	inUseCounts := map[bool]int{}

	// Precompute in-use sets per environment to avoid O(configs × workloads)
	inUseByEnv := buildConfigInUseMap(ctx, filtered)

	for _, c := range filtered {
		environmentCounts[c.EnvironmentName]++

		key := c.EnvironmentName + "/" + c.Name
		inUse := inUseByEnv[key]
		inUseCounts[inUse]++
	}

	return assembleFacets(environmentCounts, inUseCounts)
}

func buildConfigInUseMap(ctx context.Context, configs []*Config) map[string]bool {
	result := make(map[string]bool, len(configs))

	// Collect unique (teamSlug, env) pairs we need to check
	type envKey struct {
		teamSlug slug.Slug
		env      string
	}
	envs := make(map[envKey]bool)
	for _, c := range configs {
		envs[envKey{c.TeamSlug, c.EnvironmentName}] = true
	}

	// For each unique environment, list workloads once and collect referenced config names
	referencedConfigs := make(map[string]bool)
	for ek := range envs {
		apps := application.ListAllForTeamInEnvironment(ctx, ek.teamSlug, ek.env)
		for _, app := range apps {
			for _, name := range app.GetConfigs() {
				referencedConfigs[ek.env+"/"+name] = true
			}
		}

		jobs := job.ListAllForTeamInEnvironment(ctx, ek.teamSlug, ek.env)
		for _, j := range jobs {
			for _, name := range j.GetConfigs() {
				referencedConfigs[ek.env+"/"+name] = true
			}
		}
	}

	for _, c := range configs {
		key := c.EnvironmentName + "/" + c.Name
		result[key] = referencedConfigs[key]
	}

	return result
}

func assembleFacets(
	environmentCounts map[string]int,
	inUseCounts map[bool]int,
) *ConfigFacets {
	facets := &ConfigFacets{
		Environments: make([]model.EnvironmentFacetItem, 0, len(environmentCounts)),
		InUse:        make([]model.BooleanFacetItem, 0, len(inUseCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.EnvironmentFacetItem{
			EnvironmentName: env,
			Count:           count,
		})
	}

	for inUse, count := range inUseCounts {
		facets.InUse = append(facets.InUse, model.BooleanFacetItem{
			Value: inUse,
			Count: count,
		})
	}

	// Sort for stable ordering
	slices.SortFunc(facets.Environments, func(a, b model.EnvironmentFacetItem) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	slices.SortFunc(facets.InUse, func(a, b model.BooleanFacetItem) int {
		if a.Value == b.Value {
			return 0
		}
		if a.Value {
			return 1
		}
		return -1
	})

	return facets
}
