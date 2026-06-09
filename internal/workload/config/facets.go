package config

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

// Filtered returns the filtered configs, computing it exactly once per request.
func (f *ConfigFacets) Filtered(ctx context.Context) []*Config {
	f.filteredOnce.Do(func() {
		f.filteredConfigs = SortFilter.Filter(ctx, f.AllConfigs, f.Filter)
	})
	return f.filteredConfigs
}

// Environments computes environments facets for a config query.
func (f *ConfigFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllConfigs, filtered, func(c *Config) string {
		return c.EnvironmentName
	})
}

// InUse computes in-use facets for a config query.
func (f *ConfigFacets) InUse(ctx context.Context) []model.BooleanFacetItem {
	inUseCounts := map[bool]int{}
	inUseSet := buildConfigInUseSet(ctx, f.AllConfigs)

	for _, c := range f.AllConfigs {
		inUseCounts[inUseSet[c.EnvironmentName+"/"+c.Name]] = 0
	}

	filtered := f.Filtered(ctx)
	for _, c := range filtered {
		inUseCounts[inUseSet[c.EnvironmentName+"/"+c.Name]]++
	}

	inUse := make([]model.BooleanFacetItem, 0, len(inUseCounts))
	for val, count := range inUseCounts {
		inUse = append(inUse, model.BooleanFacetItem{
			Value: val,
			Count: count,
		})
	}
	model.SortBooleanFacetItems(inUse)

	return inUse
}

// Labels computes labels facets for a config query.
func (f *ConfigFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeLabelsFacet(f.AllConfigs, filtered, func(c *Config) []*model.ResourceLabel {
		return c.Labels
	})
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
