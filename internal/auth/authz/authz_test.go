package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
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
