package kafkatopic

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

func ComputeFacets(ctx context.Context, allTopics []*KafkaTopic, filter *KafkaTopicFilter) *KafkaTopicFacets {
	filtered := SortFilterTopic.Filter(ctx, allTopics, filter)

	// Seed all possible values from allTopics
	environmentCounts := map[string]int{}
	poolCounts := map[string]int{}

	for _, t := range allTopics {
		environmentCounts[t.EnvironmentName] = 0
		poolCounts[t.Pool] = 0
	}

	// Count only items matching the filter
	for _, t := range filtered {
		environmentCounts[t.EnvironmentName]++
		poolCounts[t.Pool]++
	}

	return assembleFacets(environmentCounts, poolCounts)
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
