package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graphv1/gengqlv1"
	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/persistence/bigquery"
	"github.com/nais/api/internal/persistence/bucket"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/redis"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *queryResolver) Teams(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *team.TeamOrder) (*pagination.Connection[*team.Team], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return team.List(ctx, page, orderBy)
}

func (r *queryResolver) Team(ctx context.Context, slug slug.Slug) (*team.Team, error) {
	return team.Get(ctx, slug)
}

func (r *teamResolver) AzureGroupID(ctx context.Context, obj *team.Team) (*ident.Ident, error) {
	panic(fmt.Errorf("not implemented: AzureGroupID - azureGroupID"))
}

func (r *teamResolver) Members(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *team.TeamMemberOrder) (*pagination.Connection[*team.TeamMember], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return team.ListMembers(ctx, obj.Slug, page, orderBy)
}

func (r *teamResolver) Applications(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *application.ApplicationOrder) (*pagination.Connection[*application.Application], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return application.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamResolver) Jobs(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *job.JobOrder) (*pagination.Connection[*job.Job], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return job.ListForTeam(ctx, obj.Slug, page, orderBy)
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

func (r *teamResolver) OpenSearch(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *opensearch.OpenSearchOrder) (*pagination.Connection[*opensearch.OpenSearch], error) {
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

func (r *teamResolver) ViewerIsOwner(ctx context.Context, obj *team.Team) (bool, error) {
	panic(fmt.Errorf("not implemented: ViewerIsOwner - viewerIsOwner"))
}

func (r *teamResolver) ViewerIsMember(ctx context.Context, obj *team.Team) (bool, error) {
	panic(fmt.Errorf("not implemented: ViewerIsMember - viewerIsMember"))
}

func (r *teamMemberResolver) Team(ctx context.Context, obj *team.TeamMember) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *teamMemberResolver) User(ctx context.Context, obj *team.TeamMember) (*user.User, error) {
	return user.Get(ctx, obj.UserID)
}

func (r *Resolver) Team() gengqlv1.TeamResolver { return &teamResolver{r} }

func (r *Resolver) TeamMember() gengqlv1.TeamMemberResolver { return &teamMemberResolver{r} }

type (
	teamResolver       struct{ *Resolver }
	teamMemberResolver struct{ *Resolver }
)
