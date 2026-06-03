package config

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func ComputeFacets(ctx context.Context, allConfigs []*Config, filter *ConfigFilter) *ConfigFacets {
	filtered := SortFilter.Filter(ctx, allConfigs, filter)

	environmentCounts := map[string]int{}
	inUseCounts := map[bool]int{}

	inUseSet := buildConfigInUseSet(ctx, filtered)

	for _, c := range filtered {
		environmentCounts[c.EnvironmentName]++
		inUseCounts[inUseSet[c.EnvironmentName+"/"+c.Name]]++
	}

	return assembleFacets(environmentCounts, inUseCounts)
}

func buildConfigInUseSet(ctx context.Context, configs []*Config) map[string]bool {
	if len(configs) == 0 {
		return nil
	}

	teamSlug := configs[0].TeamSlug
	envs := make(map[string]bool)
	for _, c := range configs {
		envs[c.EnvironmentName] = true
	}

	referenced := make(map[string]bool)
	for env := range envs {
		for _, app := range application.ListAllForTeamInEnvironment(ctx, teamSlug, env) {
			for _, name := range app.GetConfigs() {
				referenced[env+"/"+name] = true
			}
		}
		for _, j := range job.ListAllForTeamInEnvironment(ctx, teamSlug, env) {
			for _, name := range j.GetConfigs() {
				referenced[env+"/"+name] = true
			}
		}
	}

	return referenced
}

func assembleFacets(
	environmentCounts map[string]int,
	inUseCounts map[bool]int,
) *ConfigFacets {
	facets := &ConfigFacets{
		Environments: make([]model.StringFacetItem, 0, len(environmentCounts)),
		InUse:        make([]model.BooleanFacetItem, 0, len(inUseCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.StringFacetItem{
			Value: env,
			Count: count,
		})
	}

	for inUse, count := range inUseCounts {
		facets.InUse = append(facets.InUse, model.BooleanFacetItem{
			Value: inUse,
			Count: count,
		})
	}

	model.SortStringFacetItems(facets.Environments)
	model.SortBooleanFacetItems(facets.InUse)

	return facets
}
