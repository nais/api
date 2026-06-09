package kafkatopic

import (
	"context"

	"github.com/nais/api/internal/graph/model"
)

// Filtered returns the filtered Kafka topics, computing it exactly once per request.
func (f *KafkaTopicFacets) Filtered(ctx context.Context) []*KafkaTopic {
	f.filteredOnce.Do(func() {
		f.filteredTopics = SortFilterTopic.Filter(ctx, f.AllTopics, f.Filter)
	})
	return f.filteredTopics
}

// Environments computes environments facets for a Kafka topic query.
func (f *KafkaTopicFacets) Environments(ctx context.Context) ([]*model.StringFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeEnvironmentsFacet(f.AllTopics, filtered, func(t *KafkaTopic) string {
		return t.EnvironmentName
	})

	ret := make([]*model.StringFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}

// Pools computes pools facets for a Kafka topic query.
func (f *KafkaTopicFacets) Pools(ctx context.Context) ([]*model.StringFacetItem, error) {
	poolCounts := map[string]int{}
	for _, t := range f.AllTopics {
		poolCounts[t.Pool] = 0
	}

	filtered := f.Filtered(ctx)
	for _, t := range filtered {
		poolCounts[t.Pool]++
	}

	pools := make([]model.StringFacetItem, 0, len(poolCounts))
	for val, count := range poolCounts {
		pools = append(pools, model.StringFacetItem{
			Value: val,
			Count: count,
		})
	}
	model.SortStringFacetItems(pools)

	ret := make([]*model.StringFacetItem, len(pools))
	for i := range pools {
		ret[i] = &pools[i]
	}
	return ret, nil
}

// Labels computes labels facets for a Kafka topic query.
func (f *KafkaTopicFacets) Labels(ctx context.Context) ([]*model.LabelFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeLabelsFacet(f.AllTopics, filtered, func(t *KafkaTopic) []*model.ResourceLabel {
		return t.Labels
	})

	ret := make([]*model.LabelFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}
