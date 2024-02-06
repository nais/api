package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

const (
	authTeamCreateError = `required authorization: "teams:create"`
	authTeamUpdateError = `required authorization: "teams:update"`
)

func TestContextWithUser(t *testing.T) {
	ctx := context.Background()
	if authz.ActorFromContext(ctx) != nil {
		t.Fatal("expected nil actor")
	}

	user := &database.User{
		User: &gensql.User{
			Name:  "User Name",
			Email: "mail@example.com",
		},
	}

	roles := make([]*authz.Role, 0)

	ctx = authz.ContextWithActor(ctx, user, roles)

	want := &authz.Actor{
		User:  user,
		Roles: roles,
	}
	got := authz.ActorFromContext(ctx)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diff: -want +got\n%s", diff)
	}
}

func TestRequireGlobalAuthorization(t *testing.T) {
	user := &database.User{
		User: &gensql.User{
			Name:  "User Name",
			Email: "mail@example.com",
		},
	}

	t.Run("Nil user", func(t *testing.T) {
		if !errors.Is(authz.RequireGlobalAuthorization(nil, roles.AuthorizationTeamsCreate), authz.ErrNotAuthenticated) {
			t.Fatal("RequireGlobalAuthorization(ctx): expected ErrNotAuthenticated")
		}
	})

	t.Run("User with no roles", func(t *testing.T) {
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, []*authz.Role{}))
		if authz.RequireGlobalAuthorization(contextUser, roles.AuthorizationTeamsCreate).Error() != authTeamCreateError {
			t.Fatalf("RequireGlobalAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with insufficient roles", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				RoleName:       gensql.RoleNameTeamviewer,
				Authorizations: []roles.Authorization{},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireGlobalAuthorization(contextUser, roles.AuthorizationTeamsCreate).Error() != authTeamCreateError {
			t.Fatalf("RequireGlobalAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with sufficient role", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				RoleName:       gensql.RoleNameTeamcreator,
				Authorizations: []roles.Authorization{roles.AuthorizationTeamsCreate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireGlobalAuthorization(contextUser, roles.AuthorizationTeamsCreate) != nil {
			t.Fatal("RequireGlobalAuthorization(ctx): expected nil error")
		}
	})
}

func TestRequireAuthorizationForTeamTarget(t *testing.T) {
	user := &database.User{
		User: &gensql.User{
			Name:  "User Name",
			Email: "mail@example.com",
		},
	}
	targetTeamSlug := slug.Slug("slug")

	t.Run("Nil user", func(t *testing.T) {
		if !errors.Is(authz.RequireTeamAuthorization(nil, roles.AuthorizationTeamsCreate, targetTeamSlug), authz.ErrNotAuthenticated) {
			t.Fatal("RequireTeamAuthorization(ctx): expected ErrNotAuthenticated")
		}
	})

	t.Run("User with no roles", func(t *testing.T) {
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, []*authz.Role{}))
		if authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsCreate, targetTeamSlug).Error() != authTeamCreateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with insufficient roles", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				Authorizations: []roles.Authorization{},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsUpdate, targetTeamSlug).Error() != authTeamUpdateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamUpdateError)
		}
	})

	t.Run("User with targeted role", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				TargetTeamSlug: &targetTeamSlug,
				Authorizations: []roles.Authorization{roles.AuthorizationTeamsUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsUpdate, targetTeamSlug) != nil {
			t.Fatal("RequireTeamAuthorization(ctx): expected nil error")
		}
	})

	t.Run("User with targeted role for wrong target", func(t *testing.T) {
		wrongSlug := slug.Slug("other-team")
		userRoles := []*authz.Role{
			{
				TargetTeamSlug: &wrongSlug,
				Authorizations: []roles.Authorization{roles.AuthorizationTeamsUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsUpdate, targetTeamSlug).Error() != authTeamUpdateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamUpdateError)
		}
	})

	t.Run("User with global role", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				Authorizations: []roles.Authorization{roles.AuthorizationTeamsUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireTeamAuthorization(contextUser, roles.AuthorizationTeamsUpdate, targetTeamSlug) != nil {
			t.Fatal("RequireTeamAuthorization(ctx): expected nil error")
		}
	})
}
