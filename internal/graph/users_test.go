package graph_test

import (
	"context"
	"errors"
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
)

func TestQueryResolver_Users(t *testing.T) {
	ctx := context.Background()
	db := database.NewMockDatabase(t)
	auditLogger := auditlogger.NewAuditLoggerForTesting()
	log, _ := test.NewNullLogger()
	userSync := make(chan<- uuid.UUID)
	userSyncRuns := usersync.NewRunsHandler(5)
	resolver := graph.
		NewResolver(nil, nil, nil, nil, db, "example.com", userSync, auditLogger, nil, userSyncRuns, nil, log, nil, nil).
		Query()

	t.Run("unauthenticated user", func(t *testing.T) {
		users, err := resolver.Users(ctx, nil, nil)
		if users != nil {
			t.Fatalf("expected users to be nil, got %v", users)
		}
		if !errors.Is(err, authz.ErrNotAuthenticated) {
			t.Fatalf("expected error to be %v, got %v", authz.ErrNotAuthenticated, err)
		}
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

		p := database.Page{
			Limit:  20,
			Offset: 0,
		}
		db.EXPECT().
			GetUsers(ctx, p).
			Return([]*database.User{
				{User: &gensql.User{Email: "user1@example.com"}},
				{User: &gensql.User{Email: "user2@example.com"}},
			}, 2, nil)

		userList, err := resolver.Users(ctx, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(userList.Nodes) != 2 {
			t.Fatalf("expected 2 users, got %v", userList.Nodes)
		}
	})
}
