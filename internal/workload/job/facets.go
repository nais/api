package job

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// ComputeFacets computes facets for a job query.
func ComputeFacets(ctx context.Context, allJobs []*Job, filter *TeamJobsFilter) (*JobFacets, error) {
	environmentCounts := map[string]int{}
	stateCounts := map[JobState]int{}

	// First pass: seed with all values that exist in scope (so items with 0 matches still appear)
	for _, j := range allJobs {
		if _, ok := environmentCounts[j.EnvironmentName]; !ok {
			environmentCounts[j.EnvironmentName] = 0
		}

		state, err := GetState(ctx, j)
		if err != nil {
			state = JobStateUnknown
		}
		if _, ok := stateCounts[state]; !ok {
			stateCounts[state] = 0
		}
	}

	// Second pass: count jobs that match the full filter
	filtered := SortFilter.Filter(ctx, allJobs, filter)
	for _, j := range filtered {
		environmentCounts[j.EnvironmentName]++

		state, err := GetState(ctx, j)
		if err != nil {
			state = JobStateUnknown
		}
		stateCounts[state]++
	}

	return assembleFacets(environmentCounts, stateCounts), nil
}

func matchesFilter(ctx context.Context, j *Job, filter *TeamJobsFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(j.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, j.EnvironmentName) {
			return false
		}
	}

	if len(filter.States) > 0 {
		state, err := GetState(ctx, j)
		if err != nil {
			if !slices.Contains(filter.States, JobStateUnknown) {
				return false
			}
		} else if !slices.Contains(filter.States, state) {
			return false
		}
	}

	return true
}

func assembleFacets(environmentCounts map[string]int, stateCounts map[JobState]int) *JobFacets {
	facets := &JobFacets{
		Environments: make([]model.StringFacetItem, 0, len(environmentCounts)),
		States:       make([]JobStateFacetItem, 0, len(stateCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.StringFacetItem{
			Value: env,
			Count: count,
		})
	}

	for state, count := range stateCounts {
		facets.States = append(facets.States, JobStateFacetItem{
			State: state,
			Count: count,
		})
	}

	// Sort alphabetically for stable ordering
	slices.SortFunc(facets.Environments, func(a, b model.StringFacetItem) int {
		return strings.Compare(a.Value, b.Value)
	})

	slices.SortFunc(facets.States, func(a, b JobStateFacetItem) int {
		return strings.Compare(a.State.String(), b.State.String())
	})

	return facets
}
