package graph

import (
	"context"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
)

func (r *bigQueryDatasetResolver) Team(ctx context.Context, obj *model.BigQueryDataset) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.TeamSlug)
}

func (r *bigQueryDatasetResolver) Workload(ctx context.Context, obj *model.BigQueryDataset) (model.Workload, error) {
	return r.workload(ctx, obj.GQLVars.OwnerReference, obj.GQLVars.TeamSlug, obj.Env.Name)
}

func (r *bigQueryDatasetResolver) Cost(ctx context.Context, obj *model.BigQueryDataset) (float64, error) {
	if obj.GQLVars.OwnerReference == nil {
		return 0, nil
	}

	return r.bigQueryDatasetClient.CostForBiqQueryDataset(ctx, obj.Env.Name, obj.GQLVars.TeamSlug, obj.GQLVars.OwnerReference.Name), nil
}

func (r *bucketResolver) Team(ctx context.Context, obj *model.Bucket) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.TeamSlug)
}

func (r *bucketResolver) Workload(ctx context.Context, obj *model.Bucket) (model.Workload, error) {
	return r.workload(ctx, obj.GQLVars.OwnerReference, obj.GQLVars.TeamSlug, obj.Env.Name)
}

func (r *kafkaTopicResolver) ACL(ctx context.Context, obj *model.KafkaTopic, filter *model.KafkaTopicACLFilter, offset *int, limit *int, orderBy *model.OrderBy) (*model.KafkaTopicACLList, error) {
	acls := make([]*model.KafkaTopicACL, 0)
	if filter != nil {
		for _, acl := range obj.ACL {
			if filter.Team != nil && string(*filter.Team) != acl.TeamName && acl.TeamName != "*" {
				continue
			}
			if filter.Application != nil && *filter.Application != acl.ApplicationName && acl.ApplicationName != "*" {
				continue
			}

			acls = append(acls, acl)
		}
	} else {
		acls = obj.ACL
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(acls, func(a, b *model.KafkaTopicACL) bool {
				if a.TeamName == b.TeamName {
					return model.Compare(a.ApplicationName, b.ApplicationName, model.SortOrderAsc)
				}
				return model.Compare(a.TeamName, b.TeamName, orderBy.Direction)
			})
		case model.OrderByFieldAppName:
			model.SortWith(acls, func(a, b *model.KafkaTopicACL) bool {
				return model.Compare(a.ApplicationName, b.ApplicationName, orderBy.Direction)
			})
		case model.OrderByFieldAccess:
			model.SortWith(acls, func(a, b *model.KafkaTopicACL) bool {
				if a.Access == b.Access {
					if a.TeamName == b.TeamName {
						return model.Compare(a.ApplicationName, b.ApplicationName, model.SortOrderAsc)
					} else {
						return model.Compare(a.TeamName, b.TeamName, model.SortOrderAsc)
					}
				}
				return model.Compare(a.Access, b.Access, orderBy.Direction)
			})
		default:
			return nil, apierror.Errorf("Unknown field: %q", orderBy.Field)
		}
	}

	pagination := model.NewPagination(offset, limit)
	acls, pageInfo := model.PaginatedSlice(acls, pagination)

	return &model.KafkaTopicACLList{
		Nodes:    acls,
		PageInfo: pageInfo,
	}, nil
}

func (r *kafkaTopicResolver) Team(ctx context.Context, obj *model.KafkaTopic) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.TeamSlug)
}

func (r *kafkaTopicAclResolver) Workload(ctx context.Context, obj *model.KafkaTopicACL) (model.Workload, error) {
	env := obj.GQLVars.Env
	team := obj.TeamName
	app := obj.ApplicationName

	if r.k8sClient.AppExists(env, team, app) {
		return r.k8sClient.App(ctx, app, team, env)
	}

	if r.k8sClient.NaisJobExists(env, team, app) {
		return r.k8sClient.NaisJob(ctx, app, team, env)
	}

	altEnv := ""
	switch env {
	case "dev-gcp":
		altEnv = "dev-fss"
	case "prod-gcp":
		altEnv = "prod-fss"
	default:
		return nil, nil
	}

	if r.k8sClient.AppExists(altEnv, team, app) {
		return r.k8sClient.App(ctx, app, team, altEnv)
	}

	if r.k8sClient.NaisJobExists(altEnv, team, app) {
		return r.k8sClient.NaisJob(ctx, app, team, altEnv)
	}

	return nil, nil
}

func (r *openSearchResolver) Access(ctx context.Context, obj *model.OpenSearch) ([]*model.OpenSearchInstanceAccess, error) {
	infs, exists := r.k8sClient.Informers()[obj.Env.Name]
	if !exists {
		return nil, apierror.Errorf("Unknown env: %q", obj.Env.Name)
	}

	access, err := getOpenSearchAccess(infs.App, infs.Naisjob, obj.Name, obj.GQLVars.TeamSlug)
	if err != nil {
		return nil, apierror.Errorf("Unable to get OpenSearch instance access")
	}

	ret := make([]*model.OpenSearchInstanceAccess, len(access.Workloads))
	for i, w := range access.Workloads {
		ret[i] = &model.OpenSearchInstanceAccess{
			Role: w.Role,
			GQLVars: model.OpenSearchInstanceAccessGQLVars{
				TeamSlug:       obj.GQLVars.TeamSlug,
				OwnerReference: w.OwnerReference,
				Env:            obj.Env,
			},
		}
	}
	return ret, nil
}

func (r *openSearchResolver) Team(ctx context.Context, obj *model.OpenSearch) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.TeamSlug)
}

func (r *openSearchResolver) Cost(ctx context.Context, obj *model.OpenSearch) (float64, error) {
	if obj.GQLVars.OwnerReference == nil {
		return 0, nil
	}

	return r.openSearchClient.CostForOpenSearchInstance(ctx, obj.Env.Name, obj.GQLVars.TeamSlug, obj.GQLVars.OwnerReference.Name), nil
}

func (r *openSearchResolver) Workload(ctx context.Context, obj *model.OpenSearch) (model.Workload, error) {
	return r.workload(ctx, obj.GQLVars.OwnerReference, obj.GQLVars.TeamSlug, obj.Env.Name)
}

func (r *openSearchInstanceAccessResolver) Workload(ctx context.Context, obj *model.OpenSearchInstanceAccess) (model.Workload, error) {
	return r.workload(ctx, obj.GQLVars.OwnerReference, obj.GQLVars.TeamSlug, obj.GQLVars.Env.Name)
}

func (r *redisResolver) Access(ctx context.Context, obj *model.Redis) ([]*model.RedisInstanceAccess, error) {
	infs, exists := r.k8sClient.Informers()[obj.Env.Name]
	if !exists {
		return nil, apierror.Errorf("Unknown env: %q", obj.Env.Name)
	}

	access, err := getRedisAccess(infs.App, infs.Naisjob, obj.Name, obj.GQLVars.TeamSlug)
	if err != nil {
		return nil, apierror.Errorf("Unable to get Redis instance access")
	}

	ret := make([]*model.RedisInstanceAccess, len(access.Workloads))
	for i, w := range access.Workloads {
		ret[i] = &model.RedisInstanceAccess{
			Role: w.Role,
			GQLVars: model.RedisInstanceAccessGQLVars{
				TeamSlug:       obj.GQLVars.TeamSlug,
				OwnerReference: w.OwnerReference,
				Env:            obj.Env,
			},
		}
	}
	return ret, nil
}

func (r *redisResolver) Team(ctx context.Context, obj *model.Redis) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.TeamSlug)
}

func (r *redisResolver) Cost(ctx context.Context, obj *model.Redis) (float64, error) {
	if obj.GQLVars.OwnerReference == nil {
		return 0, nil
	}

	return r.redisClient.CostForRedisInstance(ctx, obj.Env.Name, obj.GQLVars.TeamSlug, obj.GQLVars.OwnerReference.Name), nil
}

func (r *redisResolver) Workload(ctx context.Context, obj *model.Redis) (model.Workload, error) {
	return r.workload(ctx, obj.GQLVars.OwnerReference, obj.GQLVars.TeamSlug, obj.Env.Name)
}

func (r *redisInstanceAccessResolver) Workload(ctx context.Context, obj *model.RedisInstanceAccess) (model.Workload, error) {
	return r.workload(ctx, obj.GQLVars.OwnerReference, obj.GQLVars.TeamSlug, obj.GQLVars.Env.Name)
}

func (r *sqlInstanceResolver) Database(ctx context.Context, obj *model.SQLInstance) (*model.SQLDatabase, error) {
	return r.sqlInstanceClient.SqlDatabase(obj)
}

func (r *sqlInstanceResolver) Team(ctx context.Context, obj *model.SQLInstance) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.TeamSlug)
}

func (r *sqlInstanceResolver) Users(ctx context.Context, obj *model.SQLInstance) ([]*model.SQLUser, error) {
	return r.sqlInstanceClient.SqlUsers(ctx, obj)
}

func (r *sqlInstanceResolver) Workload(ctx context.Context, obj *model.SQLInstance) (model.Workload, error) {
	return r.workload(ctx, obj.GQLVars.OwnerReference, obj.GQLVars.TeamSlug, obj.Env.Name)
}

func (r *Resolver) BigQueryDataset() gengql.BigQueryDatasetResolver {
	return &bigQueryDatasetResolver{r}
}

func (r *Resolver) Bucket() gengql.BucketResolver { return &bucketResolver{r} }

func (r *Resolver) KafkaTopic() gengql.KafkaTopicResolver { return &kafkaTopicResolver{r} }

func (r *Resolver) KafkaTopicAcl() gengql.KafkaTopicAclResolver { return &kafkaTopicAclResolver{r} }

func (r *Resolver) OpenSearch() gengql.OpenSearchResolver { return &openSearchResolver{r} }

func (r *Resolver) OpenSearchInstanceAccess() gengql.OpenSearchInstanceAccessResolver {
	return &openSearchInstanceAccessResolver{r}
}

func (r *Resolver) Redis() gengql.RedisResolver { return &redisResolver{r} }

func (r *Resolver) RedisInstanceAccess() gengql.RedisInstanceAccessResolver {
	return &redisInstanceAccessResolver{r}
}

func (r *Resolver) SqlInstance() gengql.SqlInstanceResolver { return &sqlInstanceResolver{r} }

type (
	bigQueryDatasetResolver          struct{ *Resolver }
	bucketResolver                   struct{ *Resolver }
	kafkaTopicResolver               struct{ *Resolver }
	kafkaTopicAclResolver            struct{ *Resolver }
	openSearchResolver               struct{ *Resolver }
	openSearchInstanceAccessResolver struct{ *Resolver }
	redisResolver                    struct{ *Resolver }
	redisInstanceAccessResolver      struct{ *Resolver }
	sqlInstanceResolver              struct{ *Resolver }
)
