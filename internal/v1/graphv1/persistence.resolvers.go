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
	"github.com/nais/api/internal/v1/persistence/bigquery"
	"github.com/nais/api/internal/v1/persistence/bucket"
	"github.com/nais/api/internal/v1/persistence/kafkatopic"
	"github.com/nais/api/internal/v1/persistence/opensearch"
	"github.com/nais/api/internal/v1/persistence/redis"
	"github.com/nais/api/internal/v1/persistence/sqlinstance"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *applicationResolver) BigQueryDatasets(ctx context.Context, obj *application.Application, orderBy *bigquery.BigQueryDatasetOrder) (*pagination.Connection[*bigquery.BigQueryDataset], error) {
	if obj.Spec.GCP == nil {
		return pagination.EmptyConnection[*bigquery.BigQueryDataset](), nil
	}

	return bigquery.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.GCP.BigQueryDatasets, orderBy)
}

func (r *applicationResolver) RedisInstances(ctx context.Context, obj *application.Application, orderBy *redis.RedisInstanceOrder) (*pagination.Connection[*redis.RedisInstance], error) {
	return redis.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.Redis, orderBy)
}

func (r *applicationResolver) OpenSearch(ctx context.Context, obj *application.Application) (*opensearch.OpenSearch, error) {
	return opensearch.GetForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.OpenSearch)
}

func (r *applicationResolver) Buckets(ctx context.Context, obj *application.Application, orderBy *bucket.BucketOrder) (*pagination.Connection[*bucket.Bucket], error) {
	if obj.Spec.GCP == nil {
		return pagination.EmptyConnection[*bucket.Bucket](), nil
	}

	return bucket.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.GCP.Buckets, orderBy)
}

func (r *applicationResolver) KafkaTopicAcls(ctx context.Context, obj *application.Application, orderBy *kafkatopic.KafkaTopicACLOrder) (*pagination.Connection[*kafkatopic.KafkaTopicACL], error) {
	if obj.Spec.Kafka == nil {
		return pagination.EmptyConnection[*kafkatopic.KafkaTopicACL](), nil
	}

	return kafkatopic.ListForWorkload(ctx, obj.TeamSlug, obj.Name, obj.Spec.Kafka.Pool, orderBy)
}

func (r *applicationResolver) SQLInstances(ctx context.Context, obj *application.Application, orderBy *sqlinstance.SQLInstanceOrder) (*pagination.Connection[*sqlinstance.SQLInstance], error) {
	if obj.Spec.GCP == nil || len(obj.Spec.GCP.SqlInstances) == 0 {
		return pagination.EmptyConnection[*sqlinstance.SQLInstance](), nil
	}

	return sqlinstance.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.GCP.SqlInstances, orderBy)
}

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
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bucketResolver) Team(ctx context.Context, obj *bucket.Bucket) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *bucketResolver) Environment(ctx context.Context, obj *bucket.Bucket) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bucketResolver) Workload(ctx context.Context, obj *bucket.Bucket) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *jobResolver) BigQueryDatasets(ctx context.Context, obj *job.Job, orderBy *bigquery.BigQueryDatasetOrder) (*pagination.Connection[*bigquery.BigQueryDataset], error) {
	if obj.Spec.GCP == nil {
		return pagination.EmptyConnection[*bigquery.BigQueryDataset](), nil
	}

	return bigquery.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.GCP.BigQueryDatasets, orderBy)
}

func (r *jobResolver) RedisInstances(ctx context.Context, obj *job.Job, orderBy *redis.RedisInstanceOrder) (*pagination.Connection[*redis.RedisInstance], error) {
	return redis.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.Redis, orderBy)
}

func (r *jobResolver) OpenSearch(ctx context.Context, obj *job.Job) (*opensearch.OpenSearch, error) {
	return opensearch.GetForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.OpenSearch)
}

func (r *jobResolver) Buckets(ctx context.Context, obj *job.Job, orderBy *bucket.BucketOrder) (*pagination.Connection[*bucket.Bucket], error) {
	if obj.Spec.GCP == nil {
		return pagination.EmptyConnection[*bucket.Bucket](), nil
	}
	return bucket.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.GCP.Buckets, orderBy)
}

func (r *jobResolver) KafkaTopicAcls(ctx context.Context, obj *job.Job, orderBy *kafkatopic.KafkaTopicACLOrder) (*pagination.Connection[*kafkatopic.KafkaTopicACL], error) {
	if obj.Spec.Kafka == nil {
		return pagination.EmptyConnection[*kafkatopic.KafkaTopicACL](), nil
	}

	return kafkatopic.ListForWorkload(ctx, obj.TeamSlug, obj.Name, obj.Spec.Kafka.Pool, orderBy)
}

func (r *jobResolver) SQLInstances(ctx context.Context, obj *job.Job, orderBy *sqlinstance.SQLInstanceOrder) (*pagination.Connection[*sqlinstance.SQLInstance], error) {
	if obj.Spec.GCP == nil || len(obj.Spec.GCP.SqlInstances) == 0 {
		return pagination.EmptyConnection[*sqlinstance.SQLInstance](), nil
	}

	return sqlinstance.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Spec.GCP.SqlInstances, orderBy)
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

func (r *openSearchResolver) Team(ctx context.Context, obj *opensearch.OpenSearch) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *openSearchResolver) Environment(ctx context.Context, obj *opensearch.OpenSearch) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *openSearchResolver) Workload(ctx context.Context, obj *opensearch.OpenSearch) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *openSearchResolver) Access(ctx context.Context, obj *opensearch.OpenSearch, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *opensearch.OpenSearchAccessOrder) (*pagination.Connection[*opensearch.OpenSearchAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return opensearch.ListAccess(ctx, obj, page, orderBy)
}

func (r *openSearchAccessResolver) Workload(ctx context.Context, obj *opensearch.OpenSearchAccess) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
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

func (r *redisInstanceResolver) Workload(ctx context.Context, obj *redis.RedisInstance) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *redisInstanceAccessResolver) Workload(ctx context.Context, obj *redis.RedisInstanceAccess) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
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
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *sqlInstanceResolver) Database(ctx context.Context, obj *sqlinstance.SQLInstance) (*sqlinstance.SQLDatabase, error) {
	db, err := sqlinstance.GetDatabase(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
	if err != nil {
		var errNotFound *watcher.ErrorNotFound

		if errors.As(err, &errNotFound) {
			return nil, nil
		}
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

func (r *teamResolver) BigQueryDatasets(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *bigquery.BigQueryDatasetOrder) (*pagination.Connection[*bigquery.BigQueryDataset], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return bigquery.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamResolver) RedisInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *redis.RedisInstanceOrder) (*pagination.Connection[*redis.RedisInstance], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return redis.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamResolver) OpenSearchInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *opensearch.OpenSearchOrder) (*pagination.Connection[*opensearch.OpenSearch], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return opensearch.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamResolver) Buckets(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *bucket.BucketOrder) (*pagination.Connection[*bucket.Bucket], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return bucket.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamResolver) KafkaTopics(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *kafkatopic.KafkaTopicOrder) (*pagination.Connection[*kafkatopic.KafkaTopic], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return kafkatopic.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamResolver) SQLInstances(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *sqlinstance.SQLInstanceOrder) (*pagination.Connection[*sqlinstance.SQLInstance], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return sqlinstance.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) Bucket(ctx context.Context, obj *team.TeamEnvironment, name string) (*bucket.Bucket, error) {
	return bucket.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamEnvironmentResolver) KafkaTopic(ctx context.Context, obj *team.TeamEnvironment, name string) (*kafkatopic.KafkaTopic, error) {
	return kafkatopic.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamEnvironmentResolver) OpenSearchInstance(ctx context.Context, obj *team.TeamEnvironment, name string) (*opensearch.OpenSearch, error) {
	return opensearch.Get(ctx, obj.TeamSlug, obj.Name, name)
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
