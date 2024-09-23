package graphv1

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/apierror"
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

	deleteKey, err := team.CreateDeleteKey(ctx, input.Slug, actor)
	if err != nil {
		return nil, err
	}

	return &team.RequestTeamDeletionPayload{
		Key: deleteKey,
	}, nil
}

func (r *mutationResolver) ConfirmTeamDeletion(ctx context.Context, input team.ConfirmTeamDeletionInput) (*team.ConfirmTeamDeletionPayload, error) {
	deleteKey, err := team.GetDeleteKey(ctx, input.Slug, input.Key)
	if err != nil {
		return nil, apierror.Errorf("Unknown deletion key: %q", input.Key)
	}

	actor := authz.ActorFromContext(ctx)
	if err := authz.RequireTeamRole(actor, deleteKey.TeamSlug, gensql.RoleNameTeamowner); err != nil {
		return nil, err
	}

	if actor.User.GetID() == deleteKey.CreatedByUserID {
		return nil, apierror.Errorf("You cannot confirm your own delete key.")
	}

	if deleteKey.ConfirmedAt != nil {
		return nil, apierror.Errorf("Key has already been confirmed, team is currently being deleted.")
	}

	if deleteKey.HasExpired() {
		return nil, apierror.Errorf("Team delete key has expired, you need to request a new key.")
	}

	if err := team.ConfirmDeleteKey(ctx, deleteKey.TeamSlug, input.Key, actor); err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	if err := r.triggerTeamDeletedEvent(ctx, deleteKey.TeamSlug, correlationID); err != nil {
		return nil, fmt.Errorf("failed to trigger team created event: %w", err)
	}

	return &team.ConfirmTeamDeletionPayload{
		DeletionStarted: true,
	}, nil
}

func (r *mutationResolver) AddTeamMember(ctx context.Context, input team.AddTeamMemberInput) (*team.AddTeamMemberPayload, error) {
	actor := authz.ActorFromContext(ctx)
	if err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMembersAdmin, input.TeamSlug); err != nil {
		return nil, err
	}

	_, err := team.Get(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	u, err := user.GetByEmail(ctx, input.UserEmail)
	if err != nil {
		return nil, err
	}

	input.UserID = u.UUID
	if err := team.AddMember(ctx, input, actor); err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	if err := r.triggerTeamUpdatedEvent(ctx, input.TeamSlug, correlationID); err != nil {
		return nil, err
	}

	return &team.AddTeamMemberPayload{
		Member: &team.TeamMember{
			Role:     input.Role,
			TeamSlug: input.TeamSlug,
			UserID:   u.UUID,
		},
	}, nil
}

func (r *mutationResolver) RemoveTeamMember(ctx context.Context, input team.RemoveTeamMemberInput) (*team.RemoveTeamMemberPayload, error) {
	actor := authz.ActorFromContext(ctx)
	if err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMembersAdmin, input.TeamSlug); err != nil {
		return nil, err
	}

	_, err := team.Get(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	u, err := user.GetByEmail(ctx, input.UserEmail)
	if err != nil {
		return nil, err
	}

	input.UserID = u.UUID
	if err := team.RemoveMember(ctx, input, actor); err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	if err := r.triggerTeamUpdatedEvent(ctx, input.TeamSlug, correlationID); err != nil {
		return nil, err
	}

	return &team.RemoveTeamMemberPayload{
		UserID:   u.UUID,
		TeamSlug: input.TeamSlug,
	}, nil
}

func (r *mutationResolver) SetTeamMemberRole(ctx context.Context, input team.SetTeamMemberRoleInput) (*team.SetTeamMemberRolePayload, error) {
	actor := authz.ActorFromContext(ctx)
	if err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMembersAdmin, input.TeamSlug); err != nil {
		return nil, err
	}

	_, err := team.Get(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	u, err := user.GetByEmail(ctx, input.UserEmail)
	if err != nil {
		return nil, err
	}

	input.UserID = u.UUID
	if err := team.SetMemberRole(ctx, input, actor); err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	if err := r.triggerTeamUpdatedEvent(ctx, input.TeamSlug, correlationID); err != nil {
		return nil, err
	}

	return &team.SetTeamMemberRolePayload{
		Member: &team.TeamMember{
			Role:     input.Role,
			TeamSlug: input.TeamSlug,
			UserID:   u.UUID,
		},
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

func (r *removeTeamMemberPayloadResolver) User(ctx context.Context, obj *team.RemoveTeamMemberPayload) (*user.User, error) {
	return user.Get(ctx, obj.UserID)
}

func (r *removeTeamMemberPayloadResolver) Team(ctx context.Context, obj *team.RemoveTeamMemberPayload) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *teamResolver) Members(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *team.TeamMemberOrder) (*pagination.Connection[*team.TeamMember], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return team.ListMembers(ctx, obj.Slug, page, orderBy)
}

func (r *teamResolver) ViewerIsOwner(ctx context.Context, obj *team.Team) (bool, error) {
	return team.UserIsOwner(ctx, obj.Slug, authz.ActorFromContext(ctx).User.GetID())
}

func (r *teamResolver) ViewerIsMember(ctx context.Context, obj *team.Team) (bool, error) {
	return team.UserIsMember(ctx, obj.Slug, authz.ActorFromContext(ctx).User.GetID())
}

func (r *teamResolver) Environments(ctx context.Context, obj *team.Team) ([]*team.TeamEnvironment, error) {
	return team.ListTeamEnvironments(ctx, obj.Slug)
}

func (r *teamResolver) Environment(ctx context.Context, obj *team.Team, name string) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.Slug, name)
}

func (r *teamResolver) DeleteKey(ctx context.Context, obj *team.Team, key uuid.UUID) (*team.TeamDeleteKey, error) {
	return team.GetDeleteKey(ctx, obj.Slug, key)
}

func (r *teamDeleteKeyResolver) CreatedBy(ctx context.Context, obj *team.TeamDeleteKey) (*user.User, error) {
	return user.Get(ctx, obj.CreatedByUserID)
}

func (r *teamDeleteKeyResolver) Team(ctx context.Context, obj *team.TeamDeleteKey) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
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

func (r *teamMemberAddedAuditEntryDataResolver) User(ctx context.Context, obj *team.TeamMemberAddedAuditEntryData) (*user.User, error) {
	return user.Get(ctx, obj.UserID)
}

func (r *teamMemberRemovedAuditEntryDataResolver) User(ctx context.Context, obj *team.TeamMemberRemovedAuditEntryData) (*user.User, error) {
	return user.Get(ctx, obj.UserID)
}

func (r *teamMemberSetRoleAuditEntryDataResolver) User(ctx context.Context, obj *team.TeamMemberSetRoleAuditEntryData) (*user.User, error) {
	return user.Get(ctx, obj.UserID)
}

func (r *Resolver) RemoveTeamMemberPayload() gengqlv1.RemoveTeamMemberPayloadResolver {
	return &removeTeamMemberPayloadResolver{r}
}

func (r *Resolver) Team() gengqlv1.TeamResolver { return &teamResolver{r} }

func (r *Resolver) TeamDeleteKey() gengqlv1.TeamDeleteKeyResolver { return &teamDeleteKeyResolver{r} }

func (r *Resolver) TeamEnvironment() gengqlv1.TeamEnvironmentResolver {
	return &teamEnvironmentResolver{r}
}

func (r *Resolver) TeamMember() gengqlv1.TeamMemberResolver { return &teamMemberResolver{r} }

func (r *Resolver) TeamMemberAddedAuditEntryData() gengqlv1.TeamMemberAddedAuditEntryDataResolver {
	return &teamMemberAddedAuditEntryDataResolver{r}
}

func (r *Resolver) TeamMemberRemovedAuditEntryData() gengqlv1.TeamMemberRemovedAuditEntryDataResolver {
	return &teamMemberRemovedAuditEntryDataResolver{r}
}

func (r *Resolver) TeamMemberSetRoleAuditEntryData() gengqlv1.TeamMemberSetRoleAuditEntryDataResolver {
	return &teamMemberSetRoleAuditEntryDataResolver{r}
}

type (
	removeTeamMemberPayloadResolver         struct{ *Resolver }
	teamResolver                            struct{ *Resolver }
	teamDeleteKeyResolver                   struct{ *Resolver }
	teamEnvironmentResolver                 struct{ *Resolver }
	teamMemberResolver                      struct{ *Resolver }
	teamMemberAddedAuditEntryDataResolver   struct{ *Resolver }
	teamMemberRemovedAuditEntryDataResolver struct{ *Resolver }
	teamMemberSetRoleAuditEntryDataResolver struct{ *Resolver }
)
