package secret

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func ComputeFacets(ctx context.Context, allSecrets []*Secret, filter *SecretFilter) *SecretFacets {
	filtered := SortFilter.Filter(ctx, allSecrets, filter)

	environmentCounts := map[string]int{}
	inUseCounts := map[bool]int{}

	inUseSet := buildSecretInUseSet(ctx, allSecrets)

	for _, s := range allSecrets {
		environmentCounts[s.EnvironmentName] = 0
		inUseCounts[inUseSet[s.EnvironmentName+"/"+s.Name]] = 0
	}

	for _, s := range filtered {
		environmentCounts[s.EnvironmentName]++
		inUseCounts[inUseSet[s.EnvironmentName+"/"+s.Name]]++
	}

	return assembleFacets(environmentCounts, inUseCounts)
}

func buildSecretInUseSet(ctx context.Context, secrets []*Secret) map[string]bool {
	if len(secrets) == 0 {
		return nil
	}

	teamSlug := secrets[0].TeamSlug
	envs := make(map[string]bool)
	for _, s := range secrets {
		envs[s.EnvironmentName] = true
	}

	referenced := make(map[string]bool)
	for env := range envs {
		for _, app := range application.ListAllForTeamInEnvironment(ctx, teamSlug, env) {
			for _, name := range app.GetSecrets() {
				referenced[env+"/"+name] = true
			}
		}
		for _, j := range job.ListAllForTeamInEnvironment(ctx, teamSlug, env) {
			for _, name := range j.GetSecrets() {
				referenced[env+"/"+name] = true
			}
		}
	}

	return referenced
}

func assembleFacets(
	environmentCounts map[string]int,
	inUseCounts map[bool]int,
) *SecretFacets {
	facets := &SecretFacets{
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
