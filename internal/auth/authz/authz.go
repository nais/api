package authz

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type ContextKey string

type AuthenticatedUser interface {
	GetID() uuid.UUID
	Identity() string
	IsServiceAccount() bool
}

type Role struct {
	Authorizations         []roles.Authorization
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

// RequireRole Check if an actor has a required role
func RequireRole(actor *Actor, requiredRoleName gensql.RoleName) error {
	for _, role := range actor.Roles {
		if role.RoleName == requiredRoleName {
			return nil
		}
	}

	return ErrMissingRole{role: string(requiredRoleName)}
}

// ActorFromContext Get the actor stored in the context. Requires that a middleware has stored an actor in the first
// place.
func ActorFromContext(ctx context.Context) *Actor {
	actor, _ := ctx.Value(contextKeyUser).(*Actor)
	return actor
}

// RequireGlobalAuthorization Require an actor to have a specific authorization through a globally assigned role.
func RequireGlobalAuthorization(actor *Actor, requiredAuthzName roles.Authorization) error {
	if !actor.Authenticated() {
		return ErrNotAuthenticated
	}

	authorizations := make(map[roles.Authorization]struct{})

	for _, role := range actor.Roles {
		if role.IsGlobal() {
			for _, authorization := range role.Authorizations {
				authorizations[authorization] = struct{}{}
			}
		}
	}

	return authorized(authorizations, requiredAuthzName)
}

// RequireTeamAuthorization Require an actor to have a specific authorization through a globally assigned or a correctly
// targeted role.
func RequireTeamAuthorization(actor *Actor, requiredAuthzName roles.Authorization, targetTeamSlug slug.Slug) error {
	if !actor.Authenticated() {
		return ErrNotAuthenticated
	}

	authorizations := make(map[roles.Authorization]struct{})

	for _, role := range actor.Roles {
		if role.IsGlobal() || role.TargetsTeam(targetTeamSlug) {
			for _, authorization := range role.Authorizations {
				authorizations[authorization] = struct{}{}
			}
		}
	}

	return authorized(authorizations, requiredAuthzName)
}



// authorized Check if one of the authorizations in the map matches the required authorization.
func authorized(authorizations map[roles.Authorization]struct{}, requiredAuthzName roles.Authorization) error {
	for authorization := range authorizations {
		if authorization == requiredAuthzName {
			return nil
		}
	}

	return ErrMissingAuthorization{authorization: string(requiredAuthzName)}
}
