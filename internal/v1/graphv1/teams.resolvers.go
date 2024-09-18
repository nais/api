package graphv1

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/user"
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

	// TODO: Correlation ID?
	correlationID := uuid.New()
	if err := r.triggerTeamCreatedEvent(ctx, input.Slug, correlationID); err != nil {
		return nil, fmt.Errorf("failed to trigger team created event: %w", err)
	}

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

	t, err := team.Update(ctx, &input, actor)
	if err != nil {
		return nil, err
	}

	// TODO: Correlation ID?
	correlationID := uuid.New()
	if err := r.triggerTeamUpdatedEvent(ctx, input.Slug, correlationID); err != nil {
		return nil, fmt.Errorf("failed to trigger team updated event: %w", err)
	}

	return &team.UpdateTeamPayload{
		Team: t,
	}, nil
}

func (r *mutationResolver) SynchronizeTeam(ctx context.Context, input team.SynchronizeTeamInput) (*team.SynchronizeTeamPayload, error) {
	panic(fmt.Errorf("not implemented: SynchronizeTeam - synchronizeTeam"))
}

func (r *mutationResolver) RequestTeamDeletion(ctx context.Context, input team.RequestTeamDeletionInput) (*team.RequestTeamDeletionPayload, error) {
	actor := authz.ActorFromContext(ctx)
	if err := authz.RequireTeamRole(actor, input.Slug, gensql.RoleNameTeamowner); err != nil {
		return nil, err
	}

	if _, err := team.Get(ctx, input.Slug); err != nil {
		return nil, err
	}

	deleteKey, err := team.CreateDeleteKey(ctx, input.Slug, actor.User.GetID())
	if err != nil {
		return nil, err
	}

	return &team.RequestTeamDeletionPayload{
		Key: deleteKey,
	}, nil
}

func (r *mutationResolver) ConfirmTeamDeletion(ctx context.Context, input team.ConfirmTeamDeletionInput) (*team.ConfirmTeamDeletionPayload, error) {
	panic(fmt.Errorf("not implemented: ConfirmTeamDeletion - confirmTeamDeletion"))
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

func (r *teamResolver) ViewerIsOwner(ctx context.Context, obj *team.Team) (bool, error) {
	panic(fmt.Errorf("not implemented: ViewerIsOwner - viewerIsOwner"))
}

func (r *teamResolver) ViewerIsMember(ctx context.Context, obj *team.Team) (bool, error) {
	panic(fmt.Errorf("not implemented: ViewerIsMember - viewerIsMember"))
}

func (r *teamResolver) Environments(ctx context.Context, obj *team.Team) ([]*team.TeamEnvironment, error) {
	return team.ListTeamEnvironments(ctx, obj.Slug)
}

func (r *teamResolver) Environment(ctx context.Context, obj *team.Team, name string) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.Slug, name)
}

func (r *teamDeleteKeyResolver) CreatedBy(ctx context.Context, obj *team.TeamDeleteKey) (*user.User, error) {
	panic(fmt.Errorf("not implemented: CreatedBy - createdBy"))
}

func (r *teamDeleteKeyResolver) Team(ctx context.Context, obj *team.TeamDeleteKey) (*team.Team, error) {
	panic(fmt.Errorf("not implemented: Team - team"))
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

func (r *Resolver) TeamDeleteKey() gengqlv1.TeamDeleteKeyResolver { return &teamDeleteKeyResolver{r} }

func (r *Resolver) TeamEnvironment() gengqlv1.TeamEnvironmentResolver {
	return &teamEnvironmentResolver{r}
}

func (r *Resolver) TeamMember() gengqlv1.TeamMemberResolver { return &teamMemberResolver{r} }

type (
	teamResolver            struct{ *Resolver }
	teamDeleteKeyResolver   struct{ *Resolver }
	teamEnvironmentResolver struct{ *Resolver }
	teamMemberResolver      struct{ *Resolver }
)
