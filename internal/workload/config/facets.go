package config

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

// ComputeFacets computes facets for a config query.
func ComputeFacets(ctx context.Context, allConfigs []*Config, filter *ConfigFilter) *ConfigFacets {
	environmentCounts := map[string]int{}
	inUseCounts := map[bool]int{}

	for _, c := range allConfigs {
		if !matchesFacetFilter(c, filter) {
			continue
		}
		environmentCounts[c.EnvironmentName]++

		inUse := isConfigInUse(ctx, c)
		inUseCounts[inUse]++
	}

	return assembleFacets(environmentCounts, inUseCounts)
}

func isConfigInUse(ctx context.Context, c *Config) bool {
	applications := application.ListAllForTeamInEnvironment(ctx, c.TeamSlug, c.EnvironmentName)
	for _, app := range applications {
		if slices.Contains(app.GetConfigs(), c.Name) {
			return true
		}
	}

	jobs := job.ListAllForTeamInEnvironment(ctx, c.TeamSlug, c.EnvironmentName)
	for _, j := range jobs {
		if slices.Contains(j.GetConfigs(), c.Name) {
			return true
		}
	}

	return false
}

func matchesFacetFilter(c *Config, filter *ConfigFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != nil && *filter.Name != "" {
		if !strings.Contains(strings.ToLower(c.Name), strings.ToLower(*filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, c.EnvironmentName) {
			return false
		}
	}

	return true
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
