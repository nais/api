package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/user"
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

	roles := make([]*authz.Role, 0)

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

func TestCanDeleteTeamRejectsServiceAccounts(t *testing.T) {
	actor := authz.NewMockAuthenticatedUser(t)
	actor.EXPECT().IsServiceAccount().Return(true).Once()

	ctx := authz.ContextWithActor(context.Background(), actor, nil)
	if err := authz.CanDeleteTeam(ctx, "some-team"); !errors.Is(err, authz.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestCanGrantPostgresAccessRejectsServiceAccounts(t *testing.T) {
	actor := authz.NewMockAuthenticatedUser(t)
	actor.EXPECT().IsServiceAccount().Return(true).Once()

	ctx := authz.ContextWithActor(context.Background(), actor, nil)
	if err := authz.CanGrantPostgresAccess(ctx, "some-team"); !errors.Is(err, authz.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestGlobalWebhookMutationsRequireAdmin(t *testing.T) {
	for _, action := range []struct {
		name      string
		authorize func(context.Context, *slug.Slug) error
	}{
		{"create", authz.CanCreateWebhook},
		{"update and test", authz.CanUpdateWebhook},
		{"delete", authz.CanDeleteWebhook},
	} {
		for _, actor := range []struct {
			name string
			user authz.AuthenticatedUser
			want error
		}{
			{"admin", &user.User{Admin: true}, nil},
			{"non-admin user", &user.User{}, authz.ErrUnauthorized},
			{"global service account", &serviceaccount.ServiceAccount{}, authz.ErrUnauthorized},
		} {
			t.Run(action.name+"/"+actor.name, func(t *testing.T) {
				ctx := authz.ContextWithActor(t.Context(), actor.user, []*authz.Role{{Name: "Team owner"}})
				if err := action.authorize(ctx, nil); !errors.Is(err, actor.want) {
					t.Fatalf("authorization error = %v, want %v", err, actor.want)
				}
			})
		}
	}
}
