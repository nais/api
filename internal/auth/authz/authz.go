package authz

import (
	"context"

	"github.com/google/uuid"
)

type ContextKey string

type AuthenticatedUser interface {
	GetID() uuid.UUID
	Identity() string
	IsServiceAccount() bool
	IsAdmin() bool
}

type Actor struct {
	User  AuthenticatedUser
	Roles []*Role
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
