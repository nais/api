package graphv1

import (
	"context"
	"errors"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/persistence/kafkatopic"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
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

	filteredACLs := make([]*kafkatopic.KafkaTopicACL, 0)
	if filter != nil {
		for _, acl := range obj.ACLs {
			if filter.Team != nil && string(*filter.Team) != acl.TeamName && acl.TeamName != "*" {
				continue
			}
			if filter.Workload != nil && *filter.Workload != acl.WorkloadName && acl.WorkloadName != "*" {
				continue
			}

			filteredACLs = append(filteredACLs, acl)
		}
	} else {
		filteredACLs = obj.ACLs
	}

	if orderBy != nil {
		switch orderBy.Field {
		case kafkatopic.KafkaTopicACLOrderFieldTeamSlug:
			slices.SortStableFunc(filteredACLs, func(a, b *kafkatopic.KafkaTopicACL) int {
				return modelv1.Compare(a.TeamName, b.TeamName, orderBy.Direction)
			})
		case kafkatopic.KafkaTopicACLOrderFieldAccess:
			slices.SortStableFunc(filteredACLs, func(a, b *kafkatopic.KafkaTopicACL) int {
				return modelv1.Compare(a.Access, b.Access, orderBy.Direction)
			})
		case kafkatopic.KafkaTopicACLOrderFieldConsumer:
			slices.SortStableFunc(filteredACLs, func(a, b *kafkatopic.KafkaTopicACL) int {
				return modelv1.Compare(a.WorkloadName, b.WorkloadName, orderBy.Direction)
			})
		}
	}

	ret := pagination.Slice(filteredACLs, page)
	return pagination.NewConnection(ret, page, int32(len(filteredACLs))), nil
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

	owner := &workload.Reference{
		Type: workload.TypeApplication,
		Name: obj.WorkloadName,
	}

	w, err := getWorkload(ctx, owner, slug.Slug(obj.TeamName), obj.EnvironmentName)
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

func (r *Resolver) KafkaTopic() gengqlv1.KafkaTopicResolver { return &kafkaTopicResolver{r} }

func (r *Resolver) KafkaTopicAcl() gengqlv1.KafkaTopicAclResolver { return &kafkaTopicAclResolver{r} }

type (
	kafkaTopicResolver    struct{ *Resolver }
	kafkaTopicAclResolver struct{ *Resolver }
)
