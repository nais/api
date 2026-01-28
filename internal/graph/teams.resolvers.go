package graph

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/user"
)

func (r *mutationResolver) CreateTeam(ctx context.Context, input team.CreateTeamInput) (*team.CreateTeamPayload, error) {
	actor := authz.ActorFromContext(ctx)

	if err := authz.CanCreateTeam(ctx); err != nil {
		return nil, err
	}

	if strings.HasPrefix(input.Slug.String(), "team") {
		return nil, &slug.ErrInvalidSlug{Message: "The name prefix 'team' is redundant. When you create a team, it is by definition a team. Try again with a different name, perhaps just removing the prefix?"}
	}

	if strings.HasPrefix(input.Slug.String(), "nais") && input.Slug != "nais-verification" {
		// The nais prefix is reserved for internal use, so we do not allow teams to be created with this prefix.
		// atm. `nais-verification` is the only exception, as it is created by the nais-verification service
		return nil, &slug.ErrInvalidSlug{Message: "The name prefix 'nais' is reserved."}
	}

	if strings.HasPrefix(input.Slug.String(), "pg-") {
		// The pg- prefix is reserved for internal use, so we do not allow teams to be created with this prefix.
		// These namespaces are used for Postgres databases in the environment.
		return nil, &slug.ErrInvalidSlug{Message: "The name prefix 'pg-' is reserved."}
	}

	if team.NamespaceExists(ctx, input.Slug) {
		return nil, &slug.ErrInvalidSlug{Message: "The team slug is not available."}
	}

	t, err := team.Create(ctx, &input, actor)
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	r.triggerTeamCreatedEvent(ctx, input.Slug, correlationID)

	return &team.CreateTeamPayload{
		Team: t,
	}, nil
}

func (r *mutationResolver) UpdateTeam(ctx context.Context, input team.UpdateTeamInput) (*team.UpdateTeamPayload, error) {
	actor := authz.ActorFromContext(ctx)

	if err := authz.CanUpdateTeamMetadata(ctx, input.Slug); err != nil {
		return nil, err
	}

	t, err := team.Update(ctx, &input, actor)
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	r.triggerTeamUpdatedEvent(ctx, input.Slug, correlationID)

	return &team.UpdateTeamPayload{
		Team: t,
	}, nil
}

func (r *mutationResolver) UpdateTeamEnvironment(ctx context.Context, input team.UpdateTeamEnvironmentInput) (*team.UpdateTeamEnvironmentPayload, error) {
	actor := authz.ActorFromContext(ctx)

	if err := authz.CanUpdateTeamMetadata(ctx, input.Slug); err != nil {
		return nil, err
	}

	teamEnvironment, err := team.UpdateEnvironment(ctx, &input, actor)
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	r.triggerTeamUpdatedEvent(ctx, input.Slug, correlationID)

	return &team.UpdateTeamEnvironmentPayload{
		TeamEnvironment: teamEnvironment,
	}, nil
}

func (r *mutationResolver) RequestTeamDeletion(ctx context.Context, input team.RequestTeamDeletionInput) (*team.RequestTeamDeletionPayload, error) {
	actor := authz.ActorFromContext(ctx)

	if err := authz.CanDeleteTeam(ctx, input.Slug); err != nil {
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
	keyUUID, err := uuid.Parse(input.Key)
	if err != nil {
		return nil, apierror.Errorf("Invalid delete key: %s", input.Key)
	}

	deleteKey, err := team.GetDeleteKey(ctx, input.Slug, keyUUID)
	if err != nil {
		return nil, apierror.Errorf("Unknown deletion key: %q", input.Key)
	}

	actor := authz.ActorFromContext(ctx)
	if err := authz.CanDeleteTeam(ctx, deleteKey.TeamSlug); err != nil {
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

	if err := team.ConfirmDeleteKey(ctx, deleteKey.TeamSlug, keyUUID, actor); err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	r.triggerTeamDeletedEvent(ctx, deleteKey.TeamSlug, correlationID)

	return &team.ConfirmTeamDeletionPayload{
		DeletionStarted: true,
	}, nil
}

func (r *mutationResolver) AddTeamMember(ctx context.Context, input team.AddTeamMemberInput) (*team.AddTeamMemberPayload, error) {
	actor := authz.ActorFromContext(ctx)

	if err := authz.CanManageTeamMembers(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if _, err := team.Get(ctx, input.TeamSlug); err != nil {
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
	r.triggerTeamUpdatedEvent(ctx, input.TeamSlug, correlationID)

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

	if err := authz.CanManageTeamMembers(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if _, err := team.Get(ctx, input.TeamSlug); err != nil {
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
	r.triggerTeamUpdatedEvent(ctx, input.TeamSlug, correlationID)

	return &team.RemoveTeamMemberPayload{
		UserID:   u.UUID,
		TeamSlug: input.TeamSlug,
	}, nil
}

func (r *mutationResolver) SetTeamMemberRole(ctx context.Context, input team.SetTeamMemberRoleInput) (*team.SetTeamMemberRolePayload, error) {
	actor := authz.ActorFromContext(ctx)

	if err := authz.CanManageTeamMembers(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if _, err := team.Get(ctx, input.TeamSlug); err != nil {
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
	r.triggerTeamUpdatedEvent(ctx, input.TeamSlug, correlationID)

	return &team.SetTeamMemberRolePayload{
		Member: &team.TeamMember{
			Role:     input.Role,
			TeamSlug: input.TeamSlug,
			UserID:   u.UUID,
		},
	}, nil
}

func (r *queryResolver) Teams(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *team.TeamOrder, filter *team.TeamFilter) (*pagination.Connection[*team.Team], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return team.List(ctx, page, orderBy, filter)
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

func (r *teamResolver) Member(ctx context.Context, obj *team.Team, email string) (*team.TeamMember, error) {
	return team.GetMemberByEmail(ctx, obj.Slug, email)
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

func (r *teamResolver) ViewerCanElevate(ctx context.Context, obj *team.Team) (bool, error) {
	err := authz.CanCreateElevation(ctx, obj.Slug)
	return err == nil, nil
}

func (r *teamResolver) Environments(ctx context.Context, obj *team.Team) ([]*team.TeamEnvironment, error) {
	return team.ListTeamEnvironments(ctx, obj.Slug)
}

func (r *teamResolver) Environment(ctx context.Context, obj *team.Team, name string) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.Slug, name)
}

func (r *teamResolver) DeleteKey(ctx context.Context, obj *team.Team, key string) (*team.TeamDeleteKey, error) {
	if err := authz.CanDeleteTeam(ctx, obj.Slug); err != nil {
		return nil, err
	}

	uid, err := uuid.Parse(key)
	if err != nil {
		return nil, apierror.Errorf("Invalid delete key: %s", key)
	}

	return team.GetDeleteKey(ctx, obj.Slug, uid)
}

func (r *teamResolver) InventoryCounts(ctx context.Context, obj *team.Team) (*team.TeamInventoryCounts, error) {
	return &team.TeamInventoryCounts{
		TeamSlug: obj.Slug,
	}, nil
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

func (r *updateTeamEnvironmentPayloadResolver) Environment(ctx context.Context, obj *team.UpdateTeamEnvironmentPayload) (*team.TeamEnvironment, error) {
	return obj.TeamEnvironment, nil
}

func (r *Resolver) RemoveTeamMemberPayload() gengql.RemoveTeamMemberPayloadResolver {
	return &removeTeamMemberPayloadResolver{r}
}

func (r *Resolver) Team() gengql.TeamResolver { return &teamResolver{r} }

func (r *Resolver) TeamDeleteKey() gengql.TeamDeleteKeyResolver { return &teamDeleteKeyResolver{r} }

func (r *Resolver) TeamEnvironment() gengql.TeamEnvironmentResolver {
	return &teamEnvironmentResolver{r}
}

func (r *Resolver) TeamInventoryCounts() gengql.TeamInventoryCountsResolver {
	return &teamInventoryCountsResolver{r}
}

func (r *Resolver) TeamMember() gengql.TeamMemberResolver { return &teamMemberResolver{r} }

func (r *Resolver) UpdateTeamEnvironmentPayload() gengql.UpdateTeamEnvironmentPayloadResolver {
	return &updateTeamEnvironmentPayloadResolver{r}
}

type (
	removeTeamMemberPayloadResolver      struct{ *Resolver }
	teamResolver                         struct{ *Resolver }
	teamDeleteKeyResolver                struct{ *Resolver }
	teamEnvironmentResolver              struct{ *Resolver }
	teamInventoryCountsResolver          struct{ *Resolver }
	teamMemberResolver                   struct{ *Resolver }
	updateTeamEnvironmentPayloadResolver struct{ *Resolver }
)
