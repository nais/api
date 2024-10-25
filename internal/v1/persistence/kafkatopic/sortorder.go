package kafkatopic

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/sortfilter"
)

var (
	SortFilterTopic    = sortfilter.New[*KafkaTopic, KafkaTopicOrderField, struct{}](KafkaTopicOrderFieldName)
	SortFilterTopicACL = sortfilter.New[*KafkaTopicACL, KafkaTopicACLOrderField, *KafkaTopicACLFilter](KafkaTopicACLOrderFieldTopicName)
)

func init() {
	SortFilterTopic.RegisterOrderBy(KafkaTopicOrderFieldName, func(ctx context.Context, a, b *KafkaTopic) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterTopic.RegisterOrderBy(KafkaTopicOrderFieldEnvironment, func(ctx context.Context, a, b *KafkaTopic) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterTopicACL.RegisterOrderBy(KafkaTopicACLOrderFieldTopicName, func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.TopicName, b.TopicName)
	})
	SortFilterTopicACL.RegisterOrderBy(KafkaTopicACLOrderFieldTeamSlug, func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.TeamName, b.TeamName)
	})
	SortFilterTopicACL.RegisterOrderBy(KafkaTopicACLOrderFieldAccess, func(ctx context.Context, a, b *KafkaTopicACL) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterTopicACL.RegisterOrderBy(KafkaTopicACLOrderFieldConsumer, func(ctx context.Context, a, b *KafkaTopicACL) int {
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
