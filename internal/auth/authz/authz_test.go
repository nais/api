package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/role"
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
		if !errors.Is(authz.RequireGlobalAuthorization(nil, role.AuthorizationTeamsCreate), authz.ErrNotAuthenticated) {
			t.Fatal("RequireGlobalAuthorization(ctx): expected ErrNotAuthenticated")
		}
	})

	t.Run("User with no roles", func(t *testing.T) {
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, []*authz.Role{}))
		if authz.RequireGlobalAuthorization(contextUser, role.AuthorizationTeamsCreate).Error() != authTeamCreateError {
			t.Fatalf("RequireGlobalAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with insufficient roles", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				RoleName:       gensql.RoleNameTeamviewer,
				Authorizations: []role.Authorization{},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireGlobalAuthorization(contextUser, role.AuthorizationTeamsCreate).Error() != authTeamCreateError {
			t.Fatalf("RequireGlobalAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with sufficient role", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				RoleName:       gensql.RoleNameTeamcreator,
				Authorizations: []role.Authorization{role.AuthorizationTeamsCreate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireGlobalAuthorization(contextUser, role.AuthorizationTeamsCreate) != nil {
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
		if !errors.Is(authz.RequireTeamAuthorization(nil, role.AuthorizationTeamsCreate, targetTeamSlug), authz.ErrNotAuthenticated) {
			t.Fatal("RequireTeamAuthorization(ctx): expected ErrNotAuthenticated")
		}
	})

	t.Run("User with no roles", func(t *testing.T) {
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, []*authz.Role{}))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsCreate, targetTeamSlug).Error() != authTeamCreateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamCreateError)
		}
	})

	t.Run("User with insufficient roles", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				Authorizations: []role.Authorization{},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsMetadataUpdate, targetTeamSlug).Error() != authTeamUpdateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamUpdateError)
		}
	})

	t.Run("User with targeted role", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				TargetTeamSlug: &targetTeamSlug,
				Authorizations: []role.Authorization{role.AuthorizationTeamsMetadataUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsMetadataUpdate, targetTeamSlug) != nil {
			t.Fatal("RequireTeamAuthorization(ctx): expected nil error")
		}
	})

	t.Run("User with targeted role for wrong target", func(t *testing.T) {
		wrongSlug := slug.Slug("other-team")
		userRoles := []*authz.Role{
			{
				TargetTeamSlug: &wrongSlug,
				Authorizations: []role.Authorization{role.AuthorizationTeamsMetadataUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsMetadataUpdate, targetTeamSlug).Error() != authTeamUpdateError {
			t.Fatalf("RequireTeamAuthorization(ctx): expected error text to match %q", authTeamUpdateError)
		}
	})

	t.Run("User with global role", func(t *testing.T) {
		userRoles := []*authz.Role{
			{
				Authorizations: []role.Authorization{role.AuthorizationTeamsMetadataUpdate},
			},
		}
		contextUser := authz.ActorFromContext(authz.ContextWithActor(context.Background(), user, userRoles))
		if authz.RequireTeamAuthorization(contextUser, role.AuthorizationTeamsMetadataUpdate, targetTeamSlug) != nil {
			t.Fatal("RequireTeamAuthorization(ctx): expected nil error")
		}
	})
}
