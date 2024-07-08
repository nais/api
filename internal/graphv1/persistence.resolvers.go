package graphv1

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/bucket"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/redis"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *bigQueryDatasetResolver) Team(ctx context.Context, obj *bigquery.BigQueryDataset) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *bigQueryDatasetResolver) Environment(ctx context.Context, obj *bigquery.BigQueryDataset) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bigQueryDatasetResolver) Workload(ctx context.Context, obj *bigquery.BigQueryDataset) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bigQueryDatasetResolver) Cost(ctx context.Context, obj *bigquery.BigQueryDataset) (float64, error) {
	// Should we make cost a separate domain?
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *bucketResolver) Team(ctx context.Context, obj *bucket.Bucket) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *bucketResolver) Environment(ctx context.Context, obj *bucket.Bucket) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bucketResolver) Workload(ctx context.Context, obj *bucket.Bucket) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *kafkaTopicResolver) Team(ctx context.Context, obj *kafkatopic.KafkaTopic) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *kafkaTopicResolver) Environment(ctx context.Context, obj *kafkatopic.KafkaTopic) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *kafkaTopicResolver) ACL(ctx context.Context, obj *kafkatopic.KafkaTopic, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *kafkatopic.KafkaTopicACLFilter, orderBy *kafkatopic.KafkaTopicACLOrder) (*pagination.Connection[*kafkatopic.KafkaTopicACL], error) {
	// TODO: Handle the pagination here or somewhere in the kafkatopic package?

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
			if filter.Application != nil && *filter.Application != acl.ApplicationName && acl.ApplicationName != "*" {
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
				return modelv1.Compare(a.ApplicationName, b.ApplicationName, orderBy.Direction)
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
	if obj.ApplicationName == "*" || obj.TeamName == "*" {
		return nil, nil
	}

	// TODO: Hardcoded owner kind for now, should probably be set in the toKafkaTopicACLs function in the kafkatopic package
	owner := &metav1.OwnerReference{
		Kind: "Application",
		Name: obj.ApplicationName,
	}
	return r.workload(ctx, owner, slug.Slug(obj.TeamName), obj.EnvironmentName)
}

func (r *openSearchResolver) Team(ctx context.Context, obj *opensearch.OpenSearch) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *openSearchResolver) Environment(ctx context.Context, obj *opensearch.OpenSearch) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *openSearchResolver) Access(ctx context.Context, obj *opensearch.OpenSearch) ([]*opensearch.OpenSearchAccess, error) {
	panic(fmt.Errorf("not implemented: Access - access"))
}

func (r *openSearchResolver) Cost(ctx context.Context, obj *opensearch.OpenSearch) (float64, error) {
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *redisInstanceResolver) Team(ctx context.Context, obj *redis.RedisInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *redisInstanceResolver) Environment(ctx context.Context, obj *redis.RedisInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *redisInstanceResolver) Access(ctx context.Context, obj *redis.RedisInstance) ([]*redis.RedisInstanceAccess, error) {
	panic(fmt.Errorf("not implemented: Access - access"))
}

func (r *redisInstanceResolver) Cost(ctx context.Context, obj *redis.RedisInstance) (float64, error) {
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *Resolver) BigQueryDataset() gengqlv1.BigQueryDatasetResolver {
	return &bigQueryDatasetResolver{r}
}

func (r *Resolver) Bucket() gengqlv1.BucketResolver { return &bucketResolver{r} }

func (r *Resolver) KafkaTopic() gengqlv1.KafkaTopicResolver { return &kafkaTopicResolver{r} }

func (r *Resolver) KafkaTopicAcl() gengqlv1.KafkaTopicAclResolver { return &kafkaTopicAclResolver{r} }

func (r *Resolver) OpenSearch() gengqlv1.OpenSearchResolver { return &openSearchResolver{r} }

func (r *Resolver) RedisInstance() gengqlv1.RedisInstanceResolver { return &redisInstanceResolver{r} }

type (
	bigQueryDatasetResolver struct{ *Resolver }
	bucketResolver          struct{ *Resolver }
	kafkaTopicResolver      struct{ *Resolver }
	kafkaTopicAclResolver   struct{ *Resolver }
	openSearchResolver      struct{ *Resolver }
	redisInstanceResolver   struct{ *Resolver }
)
