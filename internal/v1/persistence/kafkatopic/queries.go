package kafkatopic

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/searchv1"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*KafkaTopic, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*KafkaTopic, error) {
	return fromContext(ctx).watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *KafkaTopicOrder) (*KafkaTopicConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderTopics(all, orderBy)

	slice := pagination.Slice(all, page)
	return pagination.NewConnection(slice, page, int32(len(all))), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*KafkaTopic {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, workloadName, poolName string, orderBy *KafkaTopicACLOrder) (*KafkaTopicACLConnection, error) {
	topics := fromContext(ctx).watcher.All()
	ret := make([]*KafkaTopicACL, 0)
	for _, t := range watcher.Objects(topics) {
		if t.Pool != poolName {
			continue
		}

		for _, acl := range t.ACLs {
			if stringMatch(teamSlug.String(), acl.TeamName) && stringMatch(workloadName, acl.WorkloadName) {
				ret = append(ret, acl)
			}
		}
	}
	orderTopicACLs(ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func Search(ctx context.Context, q string) ([]*searchv1.Result, error) {
	topics := fromContext(ctx).watcher.All()

	ret := make([]*searchv1.Result, 0)
	for _, topic := range topics {
		rank := searchv1.Match(q, topic.Obj.Name)
		if searchv1.Include(rank) {
			ret = append(ret, &searchv1.Result{
				Rank: rank,
				Node: topic.Obj,
			})
		}
	}

	return ret, nil
}

func stringMatch(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	pattern = strings.Replace(pattern, "*", "", 1)
	return strings.HasPrefix(s, pattern)
}

func orderTopics(topics []*KafkaTopic, orderBy *KafkaTopicOrder) {
	if orderBy == nil {
		orderBy = &KafkaTopicOrder{
			Field:     KafkaTopicOrderFieldName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}
	switch orderBy.Field {
	case KafkaTopicOrderFieldName:
		slices.SortStableFunc(topics, func(a, b *KafkaTopic) int {
			return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
		})
	case KafkaTopicOrderFieldEnvironment:
		slices.SortStableFunc(topics, func(a, b *KafkaTopic) int {
			return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
		})
	}
}

func orderTopicACLs(topics []*KafkaTopicACL, orderBy *KafkaTopicACLOrder) {
	if orderBy == nil {
		orderBy = &KafkaTopicACLOrder{
			Field:     KafkaTopicACLOrderFieldTopicName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}
	switch orderBy.Field {
	case KafkaTopicACLOrderFieldTopicName:
		slices.SortStableFunc(topics, func(a, b *KafkaTopicACL) int {
			return modelv1.Compare(a.TopicName, b.TopicName, orderBy.Direction)
		})
	case KafkaTopicACLOrderFieldAccess:
		slices.SortStableFunc(topics, func(a, b *KafkaTopicACL) int {
			return modelv1.Compare(a.Access, b.Access, orderBy.Direction)
		})
	case KafkaTopicACLOrderFieldConsumer:
		slices.SortStableFunc(topics, func(a, b *KafkaTopicACL) int {
			return modelv1.Compare(a.WorkloadName, b.WorkloadName, orderBy.Direction)
		})
	case KafkaTopicACLOrderFieldTeamSlug:
		slices.SortStableFunc(topics, func(a, b *KafkaTopicACL) int {
			return modelv1.Compare(a.TeamName, b.TeamName, orderBy.Direction)
		})
	}
}
