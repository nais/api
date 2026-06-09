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
func (f *ConfigFacets) Environments(ctx context.Context) ([]*model.StringFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeEnvironmentsFacet(f.AllConfigs, filtered, func(c *Config) string {
		return c.EnvironmentName
	})

	ret := make([]*model.StringFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}

// InUse computes in-use facets for a config query.
func (f *ConfigFacets) InUse(ctx context.Context) ([]*model.BooleanFacetItem, error) {
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

	ret := make([]*model.BooleanFacetItem, len(inUse))
	for i := range inUse {
		ret[i] = &inUse[i]
	}
	return ret, nil
}

// Labels computes labels facets for a config query.
func (f *ConfigFacets) Labels(ctx context.Context) ([]*model.LabelFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeLabelsFacet(f.AllConfigs, filtered, func(c *Config) []*model.ResourceLabel {
		return c.Labels
	})

	ret := make([]*model.LabelFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
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
