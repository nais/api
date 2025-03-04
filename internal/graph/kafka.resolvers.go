package graph

import (
	"context"
	"errors"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) KafkaTopicAcls(ctx context.Context, obj *application.Application, orderBy *kafkatopic.KafkaTopicACLOrder) (*pagination.Connection[*kafkatopic.KafkaTopicACL], error) {
	if obj.Spec.Kafka == nil {
		return pagination.EmptyConnection[*kafkatopic.KafkaTopicACL](), nil
	}

	return kafkatopic.ListForWorkload(ctx, obj.TeamSlug, obj.Name, obj.Spec.Kafka.Pool, orderBy)
}

func (r *jobResolver) KafkaTopicAcls(ctx context.Context, obj *job.Job, orderBy *kafkatopic.KafkaTopicACLOrder) (*pagination.Connection[*kafkatopic.KafkaTopicACL], error) {
	if obj.Spec.Kafka == nil {
		return pagination.EmptyConnection[*kafkatopic.KafkaTopicACL](), nil
	}

	return kafkatopic.ListForWorkload(ctx, obj.TeamSlug, obj.Name, obj.Spec.Kafka.Pool, orderBy)
}

func (r *kafkaTopicResolver) Team(ctx context.Context, obj *kafkatopic.KafkaTopic) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *kafkaTopicResolver) Environment(ctx context.Context, obj *kafkatopic.KafkaTopic) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *kafkaTopicResolver) ACL(ctx context.Context, obj *kafkatopic.KafkaTopic, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *kafkatopic.KafkaTopicACLFilter, orderBy *kafkatopic.KafkaTopicACLOrder) (*pagination.Connection[*kafkatopic.KafkaTopicACL], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	filteredACLs := kafkatopic.SortFilterTopicACL.Filter(ctx, obj.ACLs, filter)

	if orderBy == nil {
		orderBy = &kafkatopic.KafkaTopicACLOrder{
			Field:     "TOPIC_NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	kafkatopic.SortFilterTopicACL.Sort(ctx, filteredACLs, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(filteredACLs, page)
	return pagination.NewConnection(ret, page, len(filteredACLs)), nil
}

func (r *kafkaTopicAclResolver) Team(ctx context.Context, obj *kafkatopic.KafkaTopicACL) (*team.Team, error) {
	if obj.TeamName == "*" {
		return nil, nil
	}
	return team.Get(ctx, slug.Slug(obj.TeamName))
}

func (r *kafkaTopicAclResolver) Workload(ctx context.Context, obj *kafkatopic.KafkaTopicACL) (workload.Workload, error) {
	if obj.WorkloadName == "*" || obj.TeamName == "*" {
		return nil, nil
	}

	w, err := tryWorkload(ctx, slug.Slug(obj.TeamName), obj.EnvironmentName, obj.WorkloadName)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, nil
	}
	return w, err
}

func (r *kafkaTopicAclResolver) Topic(ctx context.Context, obj *kafkatopic.KafkaTopicACL) (*kafkatopic.KafkaTopic, error) {
	return kafkatopic.Get(ctx, obj.TeamSlug, obj.EnvironmentName, obj.TopicName)
}

func (r *teamResolver) KafkaTopics(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *kafkatopic.KafkaTopicOrder) (*pagination.Connection[*kafkatopic.KafkaTopic], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return kafkatopic.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) KafkaTopic(ctx context.Context, obj *team.TeamEnvironment, name string) (*kafkatopic.KafkaTopic, error) {
	return kafkatopic.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamInventoryCountsResolver) KafkaTopics(ctx context.Context, obj *team.TeamInventoryCounts) (*kafkatopic.TeamInventoryCountKafkaTopics, error) {
	return &kafkatopic.TeamInventoryCountKafkaTopics{
		Total: len(kafkatopic.ListAllForTeam(ctx, obj.TeamSlug)),
	}, nil
}

func (r *Resolver) KafkaTopic() gengql.KafkaTopicResolver { return &kafkaTopicResolver{r} }

func (r *Resolver) KafkaTopicAcl() gengql.KafkaTopicAclResolver { return &kafkaTopicAclResolver{r} }

type (
	kafkaTopicResolver    struct{ *Resolver }
	kafkaTopicAclResolver struct{ *Resolver }
)
