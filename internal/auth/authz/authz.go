package authz

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/role"
)

type ContextKey string

type AuthenticatedUser interface {
	GetID() uuid.UUID
	Identity() string
	IsServiceAccount() bool
}

type Role struct {
	Authorizations         []role.Authorization
	RoleName               gensql.RoleName
	TargetServiceAccountID *uuid.UUID
	TargetTeamSlug         *slug.Slug
}

// IsGlobal Check if the role is globally assigned or not
func (r Role) IsGlobal() bool {
	return r.TargetServiceAccountID == nil && r.TargetTeamSlug == nil
}

// TargetsTeam Check if the role targets a specific team
func (r Role) TargetsTeam(targetsTeamSlug slug.Slug) bool {
	return r.TargetTeamSlug != nil && *r.TargetTeamSlug == targetsTeamSlug
}

// TargetsServiceAccount Check if the role targets a specific service account
func (r Role) TargetsServiceAccount(targetServiceAccountID uuid.UUID) bool {
	return r.TargetServiceAccountID != nil && *r.TargetServiceAccountID == targetServiceAccountID
}

type Actor struct {
	User  AuthenticatedUser
	Roles []*Role
}

var ErrNotAuthenticated = errors.New("not authenticated")

func (u *Actor) Authenticated() bool {
	if u == nil || u.User == nil {
		return false
	}

	return true
}

const contextKeyUser ContextKey = "actor"

// ContextWithActor Return a context with an actor attached to it.
func ContextWithActor(ctx context.Context, user AuthenticatedUser, roles []*Role) context.Context {
	return context.WithValue(ctx, contextKeyUser, &Actor{
		User:  user,
		Roles: roles,
	})
}

// ActorFromContext Get the actor stored in the context. Requires that a middleware has stored an actor in the first
// place.
func ActorFromContext(ctx context.Context) *Actor {
	actor, _ := ctx.Value(contextKeyUser).(*Actor)
	return actor
}

// RequireGlobalAuthorization Require an actor to have a specific authorization through a globally assigned role.
func RequireGlobalAuthorization(actor *Actor, requiredAuthzName role.Authorization) error {
	if !actor.Authenticated() {
		return ErrNotAuthenticated
	}

	authorizations := make(map[role.Authorization]struct{})

	for _, r := range actor.Roles {
		if r.IsGlobal() {
			for _, authorization := range r.Authorizations {
				authorizations[authorization] = struct{}{}
			}
		}
	}

	return authorized(authorizations, requiredAuthzName)
}

// RequireTeamAuthorization Require an actor to have a specific authorization through a globally assigned or a correctly
// targeted role.
func RequireTeamAuthorization(actor *Actor, requiredAuthzName role.Authorization, targetTeamSlug slug.Slug) error {
	if !actor.Authenticated() {
		return ErrNotAuthenticated
	}

	authorizations := make(map[role.Authorization]struct{})

	for _, r := range actor.Roles {
		if r.IsGlobal() || r.TargetsTeam(targetTeamSlug) {
			for _, authorization := range r.Authorizations {
				authorizations[authorization] = struct{}{}
			}
		}
	}

	return authorized(authorizations, requiredAuthzName)
}

// RequireTeamAuthorizationCtx fetches the actor from the context and checks if it has the required authorization.
func RequireTeamAuthorizationCtx(ctx context.Context, requiredAuthzName role.Authorization, targetTeamSlug slug.Slug) error {
	return RequireTeamAuthorization(ActorFromContext(ctx), requiredAuthzName, targetTeamSlug)
}

// authorized Check if one of the authorizations in the map matches the required authorization.
func authorized(authorizations map[role.Authorization]struct{}, requiredAuthzName role.Authorization) error {
	for authorization := range authorizations {
		if authorization == requiredAuthzName {
			return nil
		}
	}

	return ErrMissingAuthorization{authorization: string(requiredAuthzName)}
}
