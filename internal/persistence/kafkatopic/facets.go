package kafkatopic

import (
	"context"

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
		Environments: make([]model.StringFacetItem, 0, len(environmentCounts)),
		Pools:        make([]model.StringFacetItem, 0, len(poolCounts)),
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.StringFacetItem{
			Value: env,
			Count: count,
		})
	}

	for pool, count := range poolCounts {
		facets.Pools = append(facets.Pools, model.StringFacetItem{
			Value: pool,
			Count: count,
		})
	}

	model.SortStringFacetItems(facets.Environments)
	model.SortStringFacetItems(facets.Pools)

	return facets
}
