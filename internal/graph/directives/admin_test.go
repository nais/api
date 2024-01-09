package directives_test

import (
	"context"
	"testing"

	sqlc "github.com/nais/api/internal/database/gensql"

	"github.com/nais/api/internal/auth/authz"
	db "github.com/nais/api/internal/database"

	"github.com/nais/api/internal/graph/directives"
	"github.com/stretchr/testify/assert"
)

func TestAdmin(t *testing.T) {
	var obj interface{}
	var nextHandler func(ctx context.Context) (res interface{}, err error)

	t.Run("No user in context", func(t *testing.T) {
		nextHandler = func(ctx context.Context) (res interface{}, err error) {
			panic("Should not be executed")
		}
		_, err := directives.Admin()(context.Background(), obj, nextHandler)
		assert.EqualError(t, err, "not authenticated")
	})

	t.Run("User with no admin role", func(t *testing.T) {
		nextHandler = func(ctx context.Context) (res interface{}, err error) {
			panic("Should not be executed")
		}
		user := &db.User{}
		ctx := authz.ContextWithActor(context.Background(), user, []*db.Role{{RoleName: sqlc.RoleNameTeamcreator}})
		_, err := directives.Admin()(ctx, obj, nextHandler)
		assert.EqualError(t, err, "required role: \"Admin\"")
	})

	t.Run("User with no admin role", func(t *testing.T) {
		nextHandler = func(ctx context.Context) (res interface{}, err error) {
			return "executed", nil
		}
		user := &db.User{}
		ctx := authz.ContextWithActor(context.Background(), user, []*db.Role{{RoleName: sqlc.RoleNameAdmin}})
		result, err := directives.Admin()(ctx, obj, nextHandler)
		assert.NoError(t, err)
		assert.Equal(t, "executed", result)
	})
}
