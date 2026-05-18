package secret

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func ComputeFacets(ctx context.Context, allSecrets []*Secret, filter *SecretFilter) *SecretFacets {
	filtered := SortFilter.Filter(ctx, allSecrets, filter)

	environmentCounts := map[string]int{}
	inUseCounts := map[bool]int{}

	// Precompute in-use sets per environment to avoid O(secrets × workloads)
	inUseByEnv := buildSecretInUseMap(ctx, filtered)

	for _, s := range filtered {
		environmentCounts[s.EnvironmentName]++

		key := s.EnvironmentName + "/" + s.Name
		inUse := inUseByEnv[key]
		inUseCounts[inUse]++
	}

	return assembleFacets(environmentCounts, inUseCounts)
}

func buildSecretInUseMap(ctx context.Context, secrets []*Secret) map[string]bool {
	result := make(map[string]bool, len(secrets))

	// Collect unique (teamSlug, env) pairs we need to check
	type envKey struct {
		teamSlug slug.Slug
		env      string
	}
	envs := make(map[envKey]bool)
	for _, s := range secrets {
		envs[envKey{s.TeamSlug, s.EnvironmentName}] = true
	}

	// For each unique environment, list workloads once and collect referenced secret names
	referencedSecrets := make(map[string]bool)
	for ek := range envs {
		apps := application.ListAllForTeamInEnvironment(ctx, ek.teamSlug, ek.env)
		for _, app := range apps {
			for _, name := range app.GetSecrets() {
				referencedSecrets[ek.env+"/"+name] = true
			}
		}

		jobs := job.ListAllForTeamInEnvironment(ctx, ek.teamSlug, ek.env)
		for _, j := range jobs {
			for _, name := range j.GetSecrets() {
				referencedSecrets[ek.env+"/"+name] = true
			}
		}
	}

	for _, s := range secrets {
		key := s.EnvironmentName + "/" + s.Name
		result[key] = referencedSecrets[key]
	}

	return result
}

func assembleFacets(
	environmentCounts map[string]int,
	inUseCounts map[bool]int,
) *SecretFacets {
	facets := &SecretFacets{
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
