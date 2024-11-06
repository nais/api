package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/role/rolesql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/user"
)

const (
	authTeamCreateError = `required authorization: "teams:create"`
	authTeamUpdateError = `required authorization: "teams:metadata:update"`
)

func TestContextWithUser(t *testing.T) {
	ctx := context.Background()
	if authz.ActorFromContext(ctx) != nil {
		t.Fatal("expected nil actor")
	}

	u := &user.User{
		Name:  "User Name",
		Email: "mail@example.com",
	}

	roles := make([]*role.Role, 0)

	ctx = authz.ContextWithActor(ctx, u, roles)

	want := &authz.Actor{
		User:  u,
		Roles: roles,
	}
	got := authz.ActorFromContext(ctx)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diff: -want +got\n%s", diff)
	}
}

func TestRequireGlobalAuthorization(t *testing.T) {
	u := &user.User{
		Name:  "User Name",
		Email: "mail@example.com",
	}

	t.Run("Nil user", func(t *testing.T) {
		if !errors.Is(authz.RequireGlobalAuthorization(nil, role.AuthorizationTeamsCreate), authz.ErrNotAuthenticated) {
			t.Fatal("RequireGlobalAuthorization(ctx): expected ErrNotAuthenticated")
		}
	})

	t.Run("User with no roles", func(t *testing.T) {
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), u, []*role.Role{}))
		if authz.RequireGlobalAuthorization(contextUser, role.AuthorizationTeamsCreate).Error() != authTeamCreateError {
			t.Fatalf("RequireGlobalAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with insufficient roles", func(t *testing.T) {
		userRoles := []*role.Role{{Name: rolesql.RoleNameTeamviewer}}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), u, userRoles))
		if authz.RequireGlobalAuthorization(contextUser, role.AuthorizationTeamsCreate).Error() != authTeamCreateError {
			t.Fatalf("RequireGlobalAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with sufficient role", func(t *testing.T) {
		userRoles := []*role.Role{{Name: rolesql.RoleNameTeamcreator}}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), u, userRoles))
		if authz.RequireGlobalAuthorization(contextUser, role.AuthorizationTeamsCreate) != nil {
			t.Fatal("RequireGlobalAuthorization(ctx): expected nil error")
		}
	})
}

func TestRequireAuthorizationForTeamTarget(t *testing.T) {
	u := &user.User{
		Name:  "User Name",
		Email: "mail@example.com",
	}
	targetTeamSlug := slug.Slug("slug")

	t.Run("Nil user", func(t *testing.T) {
		if !errors.Is(authz.RequireTeamAuthorization(nil, role.AuthorizationTeamsCreate, targetTeamSlug), authz.ErrNotAuthenticated) {
			t.Fatal("RequireTeamAuthorization(ctx): expected ErrNotAuthenticated")
		}
	})

	t.Run("User with no roles", func(t *testing.T) {
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), u, []*role.Role{}))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsCreate, targetTeamSlug).Error() != authTeamCreateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with insufficient roles", func(t *testing.T) {
		userRoles := []*role.Role{{Name: rolesql.RoleNameTeamviewer}}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), u, userRoles))
		err := authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsMetadataUpdate, targetTeamSlug)
		if err.Error() != authTeamUpdateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamUpdateError)
		}
	})

	t.Run("User with targeted role", func(t *testing.T) {
		userRoles := []*role.Role{
			{
				Name:           rolesql.RoleNameTeamowner,
				TargetTeamSlug: &targetTeamSlug,
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), u, userRoles))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsMetadataUpdate, targetTeamSlug) != nil {
			t.Fatal("RequireTeamAuthorization(ctx): expected nil error")
		}
	})

	t.Run("User with targeted role for wrong target", func(t *testing.T) {
		wrongSlug := slug.Slug("other-team")
		userRoles := []*role.Role{
			{
				Name:           rolesql.RoleNameTeamowner,
				TargetTeamSlug: &wrongSlug,
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), u, userRoles))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsMetadataUpdate, targetTeamSlug).Error() != authTeamUpdateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamUpdateError)
		}
	})

	t.Run("User with global role", func(t *testing.T) {
		userRoles := []*role.Role{
			{
				Name: rolesql.RoleNameAdmin,
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), u, userRoles))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsMetadataUpdate, targetTeamSlug) != nil {
			t.Fatal("RequireTeamAuthorization(ctx): expected nil error")
		}
	})
}
