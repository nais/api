package kafkatopic

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterTopic    = sortfilter.New[*KafkaTopic, KafkaTopicOrderField, struct{}]("NAME", model.OrderDirectionAsc)
	SortFilterTopicACL = sortfilter.New[*KafkaTopicACL, KafkaTopicACLOrderField, *KafkaTopicACLFilter]("TOPIC_NAME", model.OrderDirectionAsc)
)

func init() {
	SortFilterTopic.RegisterOrderBy("NAME", func(ctx context.Context, a, b *KafkaTopic) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterTopic.RegisterOrderBy("ENVIRONMENT", func(ctx context.Context, a, b *KafkaTopic) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterTopicACL.RegisterOrderBy("TOPIC_NAME", func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.TopicName, b.TopicName)
	})
	SortFilterTopicACL.RegisterOrderBy("TEAM_SLUG", func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.TeamName, b.TeamName)
	})
	SortFilterTopicACL.RegisterOrderBy("ACCESS", func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterTopicACL.RegisterOrderBy("CONSUMER", func(ctx context.Context, a, b *KafkaTopicACL) int {
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
