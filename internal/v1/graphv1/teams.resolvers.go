package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/auditv1"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/persistence/bigquery"
	"github.com/nais/api/internal/v1/persistence/bucket"
	"github.com/nais/api/internal/v1/persistence/kafkatopic"
	"github.com/nais/api/internal/v1/persistence/opensearch"
	"github.com/nais/api/internal/v1/persistence/redis"
	"github.com/nais/api/internal/v1/persistence/sqlinstance"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/user"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *mutationResolver) CreateTeam(ctx context.Context, input team.CreateTeamInput) (*team.CreateTeamPayload, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationTeamsCreate)
	if err != nil {
		return nil, err
	}

	t, err := team.Create(ctx, &input, actor)
	if err != nil {
		return nil, err
	}

	// TODO: ?
	//	r.triggerTeamUpdatedEvent(ctx, team.Slug, correlationID)

	return &team.CreateTeamPayload{
		Team: t,
	}, nil
}

func (r *mutationResolver) UpdateTeam(ctx context.Context, input team.UpdateTeamInput) (*team.UpdateTeamPayload, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMetadataUpdate, input.Slug)
	if err != nil {
		return nil, err
	}

	t, err := team.Update(ctx, &input)
	if err != nil {
		return nil, err
	}

	/*
		TODO: implement or move into the team.Update function

		if input.Purpose != nil {
			err = r.auditor.TeamSetPurpose(ctx, actor.User, slug, *input.Purpose)
			if err != nil {
				return nil, err
			}
		}

		if input.SlackChannel != nil {
			err = r.auditor.TeamSetDefaultSlackChannel(ctx, actor.User, slug, *input.SlackChannel)
			if err != nil {
				return nil, err
			}
		}

		r.triggerTeamUpdatedEvent(ctx, team.Slug, correlationID)
	*/

	return &team.UpdateTeamPayload{
		Team: t,
	}, nil
}

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

func (r *teamResolver) Audits(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[auditv1.AuditEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return auditv1.ListForTeam(ctx, obj.Slug, page)
}

func (r *teamEnvironmentResolver) Team(ctx context.Context, obj *team.TeamEnvironment) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *teamMemberResolver) Team(ctx context.Context, obj *team.TeamMember) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *teamMemberResolver) User(ctx context.Context, obj *team.TeamMember) (*user.User, error) {
	return user.Get(ctx, obj.UserID)
}

func (r *Resolver) Team() gengqlv1.TeamResolver { return &teamResolver{r} }

func (r *Resolver) TeamEnvironment() gengqlv1.TeamEnvironmentResolver {
	return &teamEnvironmentResolver{r}
}

func (r *Resolver) TeamMember() gengqlv1.TeamMemberResolver { return &teamMemberResolver{r} }

type (
	teamResolver            struct{ *Resolver }
	teamEnvironmentResolver struct{ *Resolver }
	teamMemberResolver      struct{ *Resolver }
)
