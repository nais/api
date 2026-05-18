package secret

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

// ComputeFacets computes facets for a secret query.
func ComputeFacets(ctx context.Context, allSecrets []*Secret, filter *SecretFilter) *SecretFacets {
	environmentCounts := map[string]int{}
	inUseCounts := map[bool]int{}

	for _, s := range allSecrets {
		if !matchesFacetFilter(s, filter) {
			continue
		}
		environmentCounts[s.EnvironmentName]++

		inUse := isSecretInUse(ctx, s)
		inUseCounts[inUse]++
	}

	return assembleFacets(environmentCounts, inUseCounts)
}

func isSecretInUse(ctx context.Context, s *Secret) bool {
	applications := application.ListAllForTeam(ctx, s.TeamSlug, nil, nil)
	for _, app := range applications {
		if slices.Contains(app.GetSecrets(), s.Name) {
			return true
		}
	}

	jobs := job.ListAllForTeam(ctx, s.TeamSlug, nil, nil)
	for _, j := range jobs {
		if slices.Contains(j.GetSecrets(), s.Name) {
			return true
		}
	}

	return false
}

func matchesFacetFilter(s *Secret, filter *SecretFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(s.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, s.EnvironmentName) {
			return false
		}
	}

	return true
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
