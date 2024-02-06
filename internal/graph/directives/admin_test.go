package directives_test

import (
	"context"
	"testing"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/directives"
)

func TestAdmin(t *testing.T) {
	var obj interface{}
	var nextHandler func(ctx context.Context) (res interface{}, err error)

	t.Run("No user in context", func(t *testing.T) {
		nextHandler = func(ctx context.Context) (res interface{}, err error) {
			panic("Should not be executed")
		}
		_, err := directives.Admin()(context.Background(), obj, nextHandler)
		if err.Error() != "not authenticated" {
			t.Errorf("expected error to be 'not authenticated', got %q", err)
		}
	})

	t.Run("User with no admin role", func(t *testing.T) {
		nextHandler = func(ctx context.Context) (res interface{}, err error) {
			panic("Should not be executed")
		}
		user := &database.User{}
		ctx := authz.ContextWithActor(context.Background(), user, []*authz.Role{{RoleName: gensql.RoleNameTeamcreator}})
		_, err := directives.Admin()(ctx, obj, nextHandler)
		// assert.EqualError(t, err, "required role: \"Admin\"")
		if err.Error() != "required role: \"Admin\"" {
			t.Errorf("expected error to be 'required role: \"Admin\"', got %q", err)
		}
	})

	t.Run("User with no admin role", func(t *testing.T) {
		nextHandler = func(ctx context.Context) (res interface{}, err error) {
			return "executed", nil
		}
		user := &database.User{}
		ctx := authz.ContextWithActor(context.Background(), user, []*authz.Role{{RoleName: gensql.RoleNameAdmin}})
		result, err := directives.Admin()(ctx, obj, nextHandler)
		if err != nil {
			t.Errorf("unexpected error: %q", err)
		}
		if result != "executed" {
			t.Errorf("expected result to be 'executed', got %q", result)
		}
	})
}
