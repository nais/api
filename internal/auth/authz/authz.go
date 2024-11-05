package authz

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/role"
	"github.com/nais/api/internal/v1/role/rolesql"
)

type ContextKey string

type AuthenticatedUser interface {
	GetID() uuid.UUID
	Identity() string
	IsServiceAccount() bool
}

type Actor struct {
	User  AuthenticatedUser
	Roles []*role.Role
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
func ContextWithActor(ctx context.Context, user AuthenticatedUser, roles []*role.Role) context.Context {
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
		if r.Name == rolesql.RoleNameAdmin {
			return nil
		}

		roleAuthz, err := r.Authorizations()
		if err != nil {
			return err
		}
		if r.IsGlobal() {
			for _, authorization := range roleAuthz {
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
		if r.Name == rolesql.RoleNameAdmin {
			return nil
		}

		roleAuthz, err := r.Authorizations()
		if err != nil {
			return err
		}
		if r.IsGlobal() || r.TargetsTeam(targetTeamSlug) {
			for _, authorization := range roleAuthz {
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

func RequireGlobalAdmin(ctx context.Context) error {
	actor := ActorFromContext(ctx)
	if !actor.Authenticated() {
		return ErrNotAuthenticated
	}

	for _, r := range actor.Roles {
		if r.Name == rolesql.RoleNameAdmin {
			return nil
		}
	}

	return ErrMissingAuthorization{authorization: "global:admin"}
}
