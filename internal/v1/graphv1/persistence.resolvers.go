package graphv1

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/persistence/bigquery"
	bucket1 "github.com/nais/api/internal/v1/persistence/bucket"
	"github.com/nais/api/internal/v1/persistence/kafkatopic"
	"github.com/nais/api/internal/v1/persistence/opensearch"
	"github.com/nais/api/internal/v1/persistence/redis"
	"github.com/nais/api/internal/v1/persistence/sqlinstance"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *bigQueryDatasetResolver) Team(ctx context.Context, obj *bigquery.BigQueryDataset) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *bigQueryDatasetResolver) Environment(ctx context.Context, obj *bigquery.BigQueryDataset) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bigQueryDatasetResolver) Access(ctx context.Context, obj *bigquery.BigQueryDataset, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *bigquery.BigQueryDatasetAccessOrder) (*pagination.Connection[*bigquery.BigQueryDatasetAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case bigquery.BigQueryDatasetAccessOrderFieldRole:
			slices.SortStableFunc(obj.Access, func(a, b *bigquery.BigQueryDatasetAccess) int {
				return modelv1.Compare(a.Role, b.Role, orderBy.Direction)
			})
		case bigquery.BigQueryDatasetAccessOrderFieldEmail:
			slices.SortStableFunc(obj.Access, func(a, b *bigquery.BigQueryDatasetAccess) int {
				return modelv1.Compare(a.Email, b.Email, orderBy.Direction)
			})

		}
	}

	ret := pagination.Slice(obj.Access, page)
	return pagination.NewConnection(ret, page, int32(len(obj.Access))), nil
}

func (r *bigQueryDatasetResolver) Workload(ctx context.Context, obj *bigquery.BigQueryDataset) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bigQueryDatasetResolver) Cost(ctx context.Context, obj *bigquery.BigQueryDataset) (float64, error) {
	// Should we make cost a separate domain?
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *bucketResolver) Team(ctx context.Context, obj *bucket1.Bucket) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *bucketResolver) Environment(ctx context.Context, obj *bucket1.Bucket) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bucketResolver) Cors(ctx context.Context, obj *bucket1.Bucket, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*bucket1.BucketCors], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret := pagination.Slice(obj.Cors, page)
	return pagination.NewConnection(ret, page, int32(len(obj.Cors))), nil
}

func (r *bucketResolver) Workload(ctx context.Context, obj *bucket1.Bucket) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
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

func (r *openSearchResolver) Workload(ctx context.Context, obj *opensearch.OpenSearch) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *openSearchResolver) Access(ctx context.Context, obj *opensearch.OpenSearch, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *opensearch.OpenSearchAccessOrder) (*pagination.Connection[*opensearch.OpenSearchAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return opensearch.ListAccess(ctx, obj, page, orderBy)
}

func (r *openSearchResolver) Cost(ctx context.Context, obj *opensearch.OpenSearch) (float64, error) {
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *openSearchAccessResolver) Workload(ctx context.Context, obj *opensearch.OpenSearchAccess) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *redisInstanceResolver) Team(ctx context.Context, obj *redis.RedisInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *redisInstanceResolver) Environment(ctx context.Context, obj *redis.RedisInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *redisInstanceResolver) Access(ctx context.Context, obj *redis.RedisInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *redis.RedisInstanceAccessOrder) (*pagination.Connection[*redis.RedisInstanceAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return redis.ListAccess(ctx, obj, page, orderBy)
}

func (r *redisInstanceResolver) Cost(ctx context.Context, obj *redis.RedisInstance) (float64, error) {
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *redisInstanceResolver) Workload(ctx context.Context, obj *redis.RedisInstance) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *redisInstanceAccessResolver) Workload(ctx context.Context, obj *redis.RedisInstanceAccess) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlDatabaseResolver) Team(ctx context.Context, obj *sqlinstance.SQLDatabase) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *sqlDatabaseResolver) Environment(ctx context.Context, obj *sqlinstance.SQLDatabase) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceResolver) Team(ctx context.Context, obj *sqlinstance.SQLInstance) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *sqlInstanceResolver) Environment(ctx context.Context, obj *sqlinstance.SQLInstance) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceResolver) Workload(ctx context.Context, obj *sqlinstance.SQLInstance) (workload.Workload, error) {
	return r.workload(ctx, obj.OwnerReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceResolver) Database(ctx context.Context, obj *sqlinstance.SQLInstance) (*sqlinstance.SQLDatabase, error) {
	db, err := sqlinstance.GetDatabase(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
	if errors.Is(err, loaderv1.ErrObjectNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return db, err
}

func (r *sqlInstanceResolver) Flags(ctx context.Context, obj *sqlinstance.SQLInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*sqlinstance.SQLInstanceFlag], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret := pagination.Slice(obj.Flags, page)
	return pagination.NewConnection(ret, page, int32(len(obj.Flags))), nil
}

func (r *sqlInstanceResolver) Users(ctx context.Context, obj *sqlinstance.SQLInstance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *sqlinstance.SQLInstanceUserOrder) (*pagination.Connection[*sqlinstance.SQLInstanceUser], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return sqlinstance.ListSQLInstanceUsers(ctx, obj, page, orderBy)
}

func (r *Resolver) BigQueryDataset() gengqlv1.BigQueryDatasetResolver {
	return &bigQueryDatasetResolver{r}
}

func (r *Resolver) Bucket() gengqlv1.BucketResolver { return &bucketResolver{r} }

func (r *Resolver) KafkaTopic() gengqlv1.KafkaTopicResolver { return &kafkaTopicResolver{r} }

func (r *Resolver) KafkaTopicAcl() gengqlv1.KafkaTopicAclResolver { return &kafkaTopicAclResolver{r} }

func (r *Resolver) OpenSearch() gengqlv1.OpenSearchResolver { return &openSearchResolver{r} }

func (r *Resolver) OpenSearchAccess() gengqlv1.OpenSearchAccessResolver {
	return &openSearchAccessResolver{r}
}

func (r *Resolver) RedisInstance() gengqlv1.RedisInstanceResolver { return &redisInstanceResolver{r} }

func (r *Resolver) RedisInstanceAccess() gengqlv1.RedisInstanceAccessResolver {
	return &redisInstanceAccessResolver{r}
}

func (r *Resolver) SqlDatabase() gengqlv1.SqlDatabaseResolver { return &sqlDatabaseResolver{r} }

func (r *Resolver) SqlInstance() gengqlv1.SqlInstanceResolver { return &sqlInstanceResolver{r} }

type (
	bigQueryDatasetResolver     struct{ *Resolver }
	bucketResolver              struct{ *Resolver }
	kafkaTopicResolver          struct{ *Resolver }
	kafkaTopicAclResolver       struct{ *Resolver }
	openSearchResolver          struct{ *Resolver }
	openSearchAccessResolver    struct{ *Resolver }
	redisInstanceResolver       struct{ *Resolver }
	redisInstanceAccessResolver struct{ *Resolver }
	sqlDatabaseResolver         struct{ *Resolver }
	sqlInstanceResolver         struct{ *Resolver }
)