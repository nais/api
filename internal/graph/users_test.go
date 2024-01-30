package graph_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/usersync"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestQueryResolver_Users(t *testing.T) {
	ctx := context.Background()
	db := database.NewMockDatabase(t)
	auditLogger := auditlogger.NewAuditLoggerForTesting()
	log, _ := test.NewNullLogger()
	userSync := make(chan<- uuid.UUID)
	userSyncRuns := usersync.NewRunsHandler(5)
	resolver := graph.
		NewResolver(nil, nil, nil, nil, db, "example.com", userSync, auditLogger, nil, userSyncRuns, nil, log).
		Query()

	t.Run("unauthenticated user", func(t *testing.T) {
		users, err := resolver.Users(ctx, nil, nil)
		assert.Nil(t, users)
		assert.ErrorIs(t, err, authz.ErrNotAuthenticated)
	})

	t.Run("user with authorization", func(t *testing.T) {
		user := &database.User{
			User: &gensql.User{
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		ctx := authz.ContextWithActor(ctx, user, []*authz.Role{
			{
				Authorizations: []roles.Authorization{roles.AuthorizationUsersList},
			},
		})

		db.EXPECT().
			GetUsers(ctx, 0, 20).Return([]*database.User{
			{User: &gensql.User{Email: "user1@example.com"}},
			{User: &gensql.User{Email: "user2@example.com"}},
		}, 2, nil)

		userList, err := resolver.Users(ctx, nil, nil)
		assert.NoError(t, err)
		assert.Len(t, userList.Nodes, 2)
	})
}
