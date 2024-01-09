package graph_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	db "github.com/nais/api/internal/database"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/usersync"
	"github.com/stretchr/testify/assert"
)

func TestQueryResolver_Users(t *testing.T) {
	ctx := context.Background()
	database := db.NewMockDatabase(t)
	auditLogger := auditlogger.NewMockAuditLogger(t)
	gcpEnvironments := []string{"env"}
	log, err := logger.GetLogger("text", "info")
	assert.NoError(t, err)
	userSync := make(chan<- uuid.UUID)
	userSyncRuns := usersync.NewRunsHandler(5)
	resolver := graph.
		NewResolver(nil, database, "example.com", userSync, auditLogger, gcpEnvironments, log, userSyncRuns).
		Query()

	t.Run("unauthenticated user", func(t *testing.T) {
		users, err := resolver.Users(ctx, nil, nil)
		assert.Nil(t, users)
		assert.ErrorIs(t, err, authz.ErrNotAuthenticated)
	})

	t.Run("user with authorization", func(t *testing.T) {
		user := &db.User{
			User: &sqlc.User{
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		ctx := authz.ContextWithActor(ctx, user, []*db.Role{
			{
				Authorizations: []roles.Authorization{roles.AuthorizationUsersList},
			},
		})

		database.On("GetUsers", ctx, 0, 20).Return([]*db.User{
			{User: &sqlc.User{Email: "user1@example.com"}},
			{User: &sqlc.User{Email: "user2@example.com"}},
		}, 2, nil)

		userList, err := resolver.Users(ctx, nil, nil)
		assert.NoError(t, err)
		assert.Len(t, userList.Nodes, 2)
	})
}
