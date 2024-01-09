package authz_test

import (
	"context"
	"testing"

	"github.com/nais/api/internal/auth/authz"
	db "github.com/nais/api/internal/database"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/roles"
	"github.com/nais/api/internal/slug"
	"github.com/stretchr/testify/assert"
)

const (
	authTeamCreateError = `required authorization: "teams:create"`
	authTeamUpdateError = `required authorization: "teams:update"`
)

func TestContextWithUser(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, authz.ActorFromContext(ctx))

	user := &db.User{
		User: &sqlc.User{
			Name:  "User Name",
			Email: "mail@example.com",
		},
	}

	roles := make([]*db.Role, 0)

	ctx = authz.ContextWithActor(ctx, user, roles)
	assert.Equal(t, user, authz.ActorFromContext(ctx).User)
	assert.Equal(t, roles, authz.ActorFromContext(ctx).Roles)
}

func TestRequireGlobalAuthorization(t *testing.T) {
	user := &db.User{
		User: &sqlc.User{
			Name:  "User Name",
			Email: "mail@example.com",
		},
	}

	t.Run("Nil user", func(t *testing.T) {
		assert.ErrorIs(t, authz.RequireGlobalAuthorization(nil, roles.AuthorizationTeamsCreate), authz.ErrNotAuthenticated)
	})

	t.Run("User with no roles", func(t *testing.T) {
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, []*db.Role{}))
		assert.EqualError(t, authz.RequireGlobalAuthorization(contextUser, roles.AuthorizationTeamsCreate), authTeamCreateError)
	})

	t.Run("User with insufficient roles", func(t *testing.T) {
		userRoles := []*db.Role{
			{
				RoleName:       sqlc.RoleNameTeamviewer,
				Authorizations: []roles.Authorization{},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		assert.EqualError(t, authz.RequireGlobalAuthorization(contextUser, roles.AuthorizationTeamsCreate), authTeamCreateError)
	})

	t.Run("User with sufficient role", func(t *testing.T) {
		userRoles := []*db.Role{
			{
				RoleName:       sqlc.RoleNameTeamcreator,
				Authorizations: []roles.Authorization{roles.AuthorizationTeamsCreate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		assert.NoError(t, authz.RequireGlobalAuthorization(contextUser, roles.AuthorizationTeamsCreate))
	})
}

func TestRequireAuthorizationForTeamTarget(t *testing.T) {
	user := &db.User{
		User: &sqlc.User{
			Name:  "User Name",
			Email: "mail@example.com",
		},
	}
	targetTeamSlug := slug.Slug("slug")

	t.Run("Nil user", func(t *testing.T) {
		assert.ErrorIs(t, authz.RequireTeamAuthorization(nil, roles.AuthorizationTeamsCreate, targetTeamSlug), authz.ErrNotAuthenticated)
	})

	t.Run("User with no roles", func(t *testing.T) {
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, []*db.Role{}))
		assert.EqualError(t, authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsCreate, targetTeamSlug), authTeamCreateError)
	})

	t.Run("User with insufficient roles", func(t *testing.T) {
		userRoles := []*db.Role{
			{
				Authorizations: []roles.Authorization{},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		assert.EqualError(t, authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsUpdate, targetTeamSlug), authTeamUpdateError)
	})

	t.Run("User with targeted role", func(t *testing.T) {
		userRoles := []*db.Role{
			{
				TargetTeamSlug: &targetTeamSlug,
				Authorizations: []roles.Authorization{roles.AuthorizationTeamsUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		assert.NoError(t, authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsUpdate, targetTeamSlug))
	})

	t.Run("User with targeted role for wrong target", func(t *testing.T) {
		wrongSlug := slug.Slug("other-team")
		userRoles := []*db.Role{
			{
				TargetTeamSlug: &wrongSlug,
				Authorizations: []roles.Authorization{roles.AuthorizationTeamsUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		assert.EqualError(t, authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsUpdate, targetTeamSlug), authTeamUpdateError)
	})

	t.Run("User with global role", func(t *testing.T) {
		userRoles := []*db.Role{
			{
				Authorizations: []roles.Authorization{roles.AuthorizationTeamsUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		assert.NoError(t, authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsUpdate, targetTeamSlug))
	})
}
