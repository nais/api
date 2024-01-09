package usersync_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/nais/api/internal/types"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	db "github.com/nais/api/internal/database"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/test"
	"github.com/nais/api/internal/usersync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	admin_directory_v1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

func TestSync(t *testing.T) {
	const (
		domain           = "example.com"
		adminGroupPrefix = "console-admins"
		numRunsToStore   = 5
	)

	correlationID := uuid.New()
	syncRuns := usersync.NewRunsHandler(numRunsToStore)

	t.Run("No local users, no remote users", func(t *testing.T) {
		ctx := context.Background()

		auditLogger := auditlogger.NewMockAuditLogger(t)
		database := db.NewMockDatabase(t)
		log := logger.NewMockLogger(t)
		log.
			On("WithCorrelationID", correlationID).
			Return(log).
			Once()

		database.
			On("Transaction", mock.Anything, mock.Anything).
			Return(nil).
			Once()

		httpClient := test.NewTestHttpClient(func(req *http.Request) *http.Response {
			return test.Response("200 OK", `{"users":[]}`)
		})
		svc, err := admin_directory_v1.NewService(ctx, option.WithHTTPClient(httpClient))
		assert.NoError(t, err)

		err = usersync.
			New(database, auditLogger, adminGroupPrefix, domain, svc, log, syncRuns).
			Sync(ctx, correlationID)
		assert.NoError(t, err)
	})

	t.Run("Local users, no remote users", func(t *testing.T) {
		ctx := context.Background()
		txCtx := context.Background()

		log := logger.NewMockLogger(t)
		auditLogger := auditlogger.NewMockAuditLogger(t)
		database := db.NewMockDatabase(t)
		dbtx := db.NewMockDatabase(t)

		log.
			On("WithCorrelationID", correlationID).
			Return(log).
			Once()

		database.
			On("Transaction", ctx, mock.Anything).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(db.DatabaseTransactionFunc)
				_ = fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		user1 := &db.User{User: &sqlc.User{ID: serialUuid(1), Email: "user1@example.com", ExternalID: "123", Name: "User 1"}}
		user2 := &db.User{User: &sqlc.User{ID: serialUuid(2), Email: "user2@example.com", ExternalID: "456", Name: "User 2"}}

		dbtx.
			On("GetAllUsers", txCtx, mock.Anything, mock.Anything).
			Return([]*db.User{user1, user2}, nil).
			Once()
		dbtx.
			On("GetAllUserRoles", txCtx).
			Return([]*db.UserRole{
				{UserRole: &sqlc.UserRole{UserID: user1.ID, RoleName: sqlc.RoleNameTeamcreator}},
				{UserRole: &sqlc.UserRole{UserID: user2.ID, RoleName: sqlc.RoleNameAdmin}},
			}, nil).
			Once()
		dbtx.
			On("DeleteUser", txCtx, user1.ID).
			Return(nil).
			Once()
		dbtx.
			On("DeleteUser", txCtx, user2.ID).
			Return(nil).
			Once()

		auditLogger.EXPECT().
			Logf(ctx, targetIdentifier(user1.Email), auditAction(types.AuditActionUsersyncDelete), `Local user deleted: "user1@example.com", external ID: "123"`).
			Return().
			Once()
		auditLogger.EXPECT().
			Logf(ctx, targetIdentifier(user2.Email), auditAction(types.AuditActionUsersyncDelete), `Local user deleted: "user2@example.com", external ID: "456"`).
			Return().
			Once()

		httpClient := test.NewTestHttpClient(
			// users
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"users":[]}`)
			},
			// admin group members
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"members":[]}`)
			},
		)
		svc, err := admin_directory_v1.NewService(ctx, option.WithHTTPClient(httpClient))
		assert.NoError(t, err)

		err = usersync.
			New(database, auditLogger, adminGroupPrefix, domain, svc, log, syncRuns).
			Sync(ctx, correlationID)
		assert.NoError(t, err)
	})

	t.Run("Create, update and delete users", func(t *testing.T) {
		ctx := context.Background()
		txCtx := context.Background()

		auditLogger := auditlogger.NewMockAuditLogger(t)
		database := db.NewMockDatabase(t)
		dbtx := db.NewMockDatabase(t)
		log := logger.NewMockLogger(t)
		log.
			On("WithCorrelationID", correlationID).
			Return(log).
			Once()
		log.
			On("Errorf", mock.AnythingOfType("string"), "unknown-admin@example.com").
			Return(nil).
			Once()

		numDefaultRoleNames := len(usersync.DefaultRoleNames)

		localUserID1 := serialUuid(1)
		localUserID2 := serialUuid(2)
		localUserID3 := serialUuid(3)
		localUserID4 := serialUuid(4)

		localUserWithIncorrectName := &db.User{User: &sqlc.User{ID: localUserID1, Email: "user1@example.com", ExternalID: "123", Name: "Incorrect Name"}}
		localUserWithCorrectName := &db.User{User: &sqlc.User{ID: localUserID1, Email: "user1@example.com", ExternalID: "123", Name: "Correct Name"}}

		localUserWithIncorrectEmail := &db.User{User: &sqlc.User{ID: localUserID2, Email: "user-123@example.com", ExternalID: "789", Name: "Some Name"}}
		localUserWithCorrectEmail := &db.User{User: &sqlc.User{ID: localUserID2, Email: "user3@example.com", ExternalID: "789", Name: "Some Name"}}

		localUserThatWillBeDeleted := &db.User{User: &sqlc.User{ID: localUserID3, Email: "delete-me@example.com", ExternalID: "321", Name: "Delete Me"}}

		createdLocalUser := &db.User{User: &sqlc.User{ID: localUserID4, Email: "user2@example.com", ExternalID: "456", Name: "Create Me"}}

		httpClient := test.NewTestHttpClient(
			// org users
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"users":[`+
					`{"id": "123", "primaryEmail":"user1@example.com","name":{"fullName":"Correct Name"}},`+ // Will update name of local user
					`{"id": "456", "primaryEmail":"user2@example.com","name":{"fullName":"Create Me"}},`+ // Will be created
					`{"id": "789", "primaryEmail":"user3@example.com","name":{"fullName":"Some Name"}}]}`) // Will update email of local user
			},
			// admin group members
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"members":[`+
					`{"id": "456", "email":"user2@example.com", "status": "ACTIVE", "type": "USER"},`+ // Will be granted admin role
					`{"Id": "666", "email":"some-group@example.com", "type": "GROUP"},`+ // Group type, will be ignored
					`{"Id": "111", "email":"unknown-admin@example.com", "status": "ACTIVE", "type": "USER"},`+ // Unknown user, will be logged
					`{"Id": "789", "email":"inactive-user@example.com", "status":"SUSPENDED", "type": "USER"}]}`) // Invalid status, will be ignored
			},
		)
		svc, err := admin_directory_v1.NewService(ctx, option.WithHTTPClient(httpClient))
		assert.NoError(t, err)

		database.
			On("Transaction", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(db.DatabaseTransactionFunc)
				_ = fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		dbtx.
			On("GetAllUserRoles", txCtx).
			Return([]*db.UserRole{
				{UserRole: &sqlc.UserRole{UserID: localUserID1, RoleName: sqlc.RoleNameTeamcreator}},
				{UserRole: &sqlc.UserRole{UserID: localUserID1, RoleName: sqlc.RoleNameAdmin}},
				{UserRole: &sqlc.UserRole{UserID: localUserID2, RoleName: sqlc.RoleNameTeamviewer}},
			}, nil).
			Once()

		dbtx.
			On("GetAllUsers", txCtx, mock.Anything, mock.Anything).
			Return([]*db.User{
				localUserWithIncorrectName,
				localUserWithIncorrectEmail,
				localUserThatWillBeDeleted,
			}, nil).
			Once()

		// user1@example.com
		dbtx.
			On("UpdateUser", txCtx, localUserWithIncorrectName.ID, "Correct Name", "user1@example.com", "123").
			Return(localUserWithCorrectName, nil).
			Once()
		dbtx.
			On("AssignGlobalRoleToUser", txCtx, localUserWithCorrectName.ID, mock.MatchedBy(func(roleName sqlc.RoleName) bool {
				return roleName != sqlc.RoleNameTeamcreator
			})).
			Return(nil).
			Times(numDefaultRoleNames - 1)

		// user2@example.com
		dbtx.
			On("CreateUser", txCtx, "Create Me", "user2@example.com", "456").
			Return(createdLocalUser, nil).
			Once()
		dbtx.
			On("AssignGlobalRoleToUser", txCtx, createdLocalUser.ID, mock.AnythingOfType("sqlc.RoleName")).
			Return(nil).
			Times(numDefaultRoleNames)

		// user3@example.com
		dbtx.
			On("UpdateUser", txCtx, localUserWithIncorrectEmail.ID, "Some Name", "user3@example.com", "789").
			Return(localUserWithCorrectEmail, nil).
			Once()
		dbtx.
			On("AssignGlobalRoleToUser", txCtx, localUserWithCorrectEmail.ID, mock.MatchedBy(func(roleName sqlc.RoleName) bool {
				return roleName != sqlc.RoleNameTeamviewer
			})).
			Return(nil).
			Times(numDefaultRoleNames - 1)

		dbtx.
			On("DeleteUser", txCtx, localUserThatWillBeDeleted.ID).
			Return(nil).
			Once()

		dbtx.
			On("AssignGlobalRoleToUser", txCtx, createdLocalUser.ID, sqlc.RoleNameAdmin).
			Return(nil).
			Once()

		dbtx.
			On("RevokeGlobalUserRole", txCtx, localUserID1, sqlc.RoleNameAdmin).
			Return(nil).
			Once()

		auditLogger.EXPECT().
			Logf(ctx, targetIdentifier("user1@example.com"), auditAction(types.AuditActionUsersyncUpdate), `Local user updated: "user1@example.com", external ID: "123"`).
			Return().
			Once()
		auditLogger.EXPECT().
			Logf(ctx, targetIdentifier("user2@example.com"), auditAction(types.AuditActionUsersyncCreate), `Local user created: "user2@example.com", external ID: "456"`).
			Return().
			Once()
		auditLogger.EXPECT().
			Logf(ctx, targetIdentifier("user3@example.com"), auditAction(types.AuditActionUsersyncUpdate), `Local user updated: "user3@example.com", external ID: "789"`).
			Return().
			Once()
		auditLogger.EXPECT().
			Logf(ctx, targetIdentifier("delete-me@example.com"), auditAction(types.AuditActionUsersyncDelete), `Local user deleted: "delete-me@example.com", external ID: "321"`).
			Return().
			Once()
		auditLogger.EXPECT().
			Logf(ctx, targetIdentifier("user2@example.com"), auditAction(types.AuditActionUsersyncAssignAdminRole), `Assign global admin role to user: "user2@example.com"`).
			Return().
			Once()
		auditLogger.EXPECT().
			Logf(ctx, targetIdentifier("user1@example.com"), auditAction(types.AuditActionUsersyncRevokeAdminRole), `Revoke global admin role from user: "user1@example.com"`).
			Return().
			Once()

		err = usersync.
			New(database, auditLogger, adminGroupPrefix, domain, svc, log, syncRuns).
			Sync(ctx, correlationID)
		assert.NoError(t, err)
	})
}

func targetIdentifier(identifier string) interface{} {
	return mock.MatchedBy(func(t []auditlogger.Target) bool {
		return t[0].Identifier == identifier
	})
}

func auditAction(action types.AuditAction) interface{} {
	return mock.MatchedBy(func(f auditlogger.Fields) bool {
		return f.Action == action
	})
}

func serialUuid(serial byte) uuid.UUID {
	return uuid.UUID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, serial}
}
