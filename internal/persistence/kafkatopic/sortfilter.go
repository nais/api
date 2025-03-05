package kafkatopic

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterTopic    = sortfilter.New[*KafkaTopic, KafkaTopicOrderField, struct{}]()
	SortFilterTopicACL = sortfilter.New[*KafkaTopicACL, KafkaTopicACLOrderField, *KafkaTopicACLFilter]()
)

func init() {
	SortFilterTopic.RegisterSort("NAME", func(ctx context.Context, a, b *KafkaTopic) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterTopic.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *KafkaTopic) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterTopicACL.RegisterSort("TOPIC_NAME", func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.TopicName, b.TopicName)
	})
	SortFilterTopicACL.RegisterSort("TEAM_SLUG", func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.TeamName, b.TeamName)
	})
	SortFilterTopicACL.RegisterSort("ACCESS", func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterTopicACL.RegisterSort("CONSUMER", func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.WorkloadName, b.WorkloadName)
	})

	SortFilterTopicACL.RegisterFilter(func(ctx context.Context, v *KafkaTopicACL, filter *KafkaTopicACLFilter) bool {
		if filter.Team != nil && string(*filter.Team) != v.TeamName && v.TeamName != "*" {
			return false
		}

		if filter.Workload != nil && *filter.Workload != v.WorkloadName && v.WorkloadName != "*" {
			return false
		}

		return true
	})
}
