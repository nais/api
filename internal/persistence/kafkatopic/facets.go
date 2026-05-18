package kafkatopic

import (
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// ComputeFacets computes facets for a Kafka topic query.
// All possible values are seeded from allTopics, but only items matching the filter are counted.
func ComputeFacets(allTopics []*KafkaTopic, filter *KafkaTopicFilter) *KafkaTopicFacets {
	// Seed all possible values from allTopics
	environmentCounts := map[string]int{}
	poolCounts := map[string]int{}

	for _, t := range allTopics {
		environmentCounts[t.EnvironmentName] = 0
		poolCounts[t.Pool] = 0
	}

	// Count only items matching the filter
	for _, t := range allTopics {
		if !matchesFilter(t, filter) {
			continue
		}
		environmentCounts[t.EnvironmentName]++
		poolCounts[t.Pool]++
	}

	return assembleFacets(environmentCounts, poolCounts)
}

// matchesFilter checks if a single topic matches the given filter.
func matchesFilter(t *KafkaTopic, filter *KafkaTopicFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(t.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, t.EnvironmentName) {
			return false
		}
	}

	if len(filter.Pools) > 0 {
		if !slices.Contains(filter.Pools, t.Pool) {
			return false
		}
	}

	return true
}

func assembleFacets(environmentCounts map[string]int, poolCounts map[string]int) *KafkaTopicFacets {
	facets := &KafkaTopicFacets{
		Environments: make([]model.EnvironmentFacetItem, 0, len(environmentCounts)),
		Pools:        make([]KafkaTopicPoolFacetItem, 0, len(poolCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.EnvironmentFacetItem{
			EnvironmentName: env,
			Count:           count,
		})
	}

	for pool, count := range poolCounts {
		facets.Pools = append(facets.Pools, KafkaTopicPoolFacetItem{
			Pool:  pool,
			Count: count,
		})
	}

	slices.SortFunc(facets.Environments, func(a, b model.EnvironmentFacetItem) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	slices.SortFunc(facets.Pools, func(a, b KafkaTopicPoolFacetItem) int {
		return strings.Compare(a.Pool, b.Pool)
	})

	return facets
}
