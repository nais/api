package usersync_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/test"
	"github.com/nais/api/internal/usersync"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/mock"
	admin_directory_v1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

func TestSync(t *testing.T) {
	const (
		domain           = "example.com"
		adminGroupPrefix = "nais-admins"
		numRunsToPersist = 5
	)

	correlationID := uuid.New()
	syncRuns := usersync.NewRunsHandler(numRunsToPersist)

	t.Run("No local users, no remote users", func(t *testing.T) {
		ctx := context.Background()

		log, _ := logrustest.NewNullLogger()
		db := database.NewMockDatabase(t)

		db.EXPECT().
			Transaction(ctx, mock.Anything).
			Return(nil).
			Once()

		auditLogger := auditlogger.New(db, log)
		httpClient := test.NewTestHttpClient(func(req *http.Request) *http.Response {
			return test.Response("200 OK", `{"users":[]}`)
		})
		svc, err := admin_directory_v1.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			t.Fatal(err)
		}

		err = usersync.
			New(db, auditLogger, adminGroupPrefix, domain, svc, log, syncRuns).
			Sync(ctx, correlationID)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Local users, no remote users", func(t *testing.T) {
		ctx := context.Background()
		txCtx := context.Background()

		log, _ := logrustest.NewNullLogger()
		db := database.NewMockDatabase(t)
		auditLogger := auditlogger.New(db, log)
		dbtx := database.NewMockDatabase(t)

		db.EXPECT().
			Transaction(ctx, mock.Anything).
			Run(func(ctx context.Context, fn database.DatabaseTransactionFunc) {
				_ = fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		user1 := &database.User{User: &gensql.User{ID: uuid.New(), Email: "user1@example.com", ExternalID: "123", Name: "User 1"}}
		user2 := &database.User{User: &gensql.User{ID: uuid.New(), Email: "user2@example.com", ExternalID: "456", Name: "User 2"}}

		for _, user := range []*database.User{user1, user2} {
			var actor *string
			db.EXPECT().
				CreateAuditLogEntry(
					ctx,
					mock.Anything,
					actor,
					audittype.AuditLogsTargetTypeUser,
					user.Email,
					audittype.AuditActionUsersyncDelete,
					fmt.Sprintf("Local user deleted: %q, external ID: %q", user.Email, user.ExternalID),
				).
				Return(nil).
				Once()
		}

		p := database.Page{
			Limit:  100,
			Offset: 0,
		}

		dbtx.EXPECT().
			GetUsers(txCtx, p).
			Return([]*database.User{user1, user2}, 2, nil).
			Once()
		dbtx.EXPECT().
			GetAllUserRoles(txCtx).
			Return([]*database.UserRole{
				{UserRole: &gensql.UserRole{UserID: user1.ID, RoleName: gensql.RoleNameTeamcreator}},
				{UserRole: &gensql.UserRole{UserID: user2.ID, RoleName: gensql.RoleNameAdmin}},
			}, nil).
			Once()
		dbtx.EXPECT().
			DeleteUser(txCtx, user1.ID).
			Return(nil).
			Once()
		dbtx.EXPECT().
			DeleteUser(txCtx, user2.ID).
			Return(nil).
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
		if err != nil {
			t.Fatal(err)
		}

		err = usersync.
			New(db, auditLogger, adminGroupPrefix, domain, svc, log, syncRuns).
			Sync(ctx, correlationID)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Create, update and delete users", func(t *testing.T) {
		ctx := context.Background()
		txCtx := context.Background()

		log, _ := logrustest.NewNullLogger()
		db := database.NewMockDatabase(t)
		auditLogger := auditlogger.New(db, log)
		dbtx := database.NewMockDatabase(t)

		numDefaultRoleNames := len(usersync.DefaultRoleNames)

		localUserID1 := uuid.New()
		localUserID2 := uuid.New()
		localUserID3 := uuid.New()
		localUserID4 := uuid.New()

		localUserWithIncorrectName := &database.User{User: &gensql.User{ID: localUserID1, Email: "user1@example.com", ExternalID: "123", Name: "Incorrect Name"}}
		localUserWithCorrectName := &database.User{User: &gensql.User{ID: localUserID1, Email: "user1@example.com", ExternalID: "123", Name: "Correct Name"}}

		localUserWithIncorrectEmail := &database.User{User: &gensql.User{ID: localUserID2, Email: "user-123@example.com", ExternalID: "789", Name: "Some Name"}}
		localUserWithCorrectEmail := &database.User{User: &gensql.User{ID: localUserID2, Email: "user3@example.com", ExternalID: "789", Name: "Some Name"}}

		localUserThatWillBeDeleted := &database.User{User: &gensql.User{ID: localUserID3, Email: "delete-me@example.com", ExternalID: "321", Name: "Delete Me"}}

		createdLocalUser := &database.User{User: &gensql.User{ID: localUserID4, Email: "user2@example.com", ExternalID: "456", Name: "Create Me"}}

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
		if err != nil {
			t.Fatal(err)
		}

		db.EXPECT().
			Transaction(mock.Anything, mock.Anything).
			Run(func(_ context.Context, fn database.DatabaseTransactionFunc) {
				_ = fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		dbtx.EXPECT().
			GetAllUserRoles(txCtx).
			Return([]*database.UserRole{
				{UserRole: &gensql.UserRole{UserID: localUserID1, RoleName: gensql.RoleNameTeamcreator}},
				{UserRole: &gensql.UserRole{UserID: localUserID1, RoleName: gensql.RoleNameAdmin}},
				{UserRole: &gensql.UserRole{UserID: localUserID2, RoleName: gensql.RoleNameTeamviewer}},
			}, nil).
			Once()

		p := database.Page{
			Limit:  100,
			Offset: 0,
		}

		dbtx.EXPECT().
			GetUsers(txCtx, p).
			Return([]*database.User{
				localUserWithIncorrectName,
				localUserWithIncorrectEmail,
				localUserThatWillBeDeleted,
			}, 3, nil).
			Once()

		// user1@example.com
		dbtx.EXPECT().
			UpdateUser(txCtx, localUserWithIncorrectName.ID, "Correct Name", "user1@example.com", "123").
			Return(localUserWithCorrectName, nil).
			Once()
		dbtx.EXPECT().
			AssignGlobalRoleToUser(txCtx, localUserWithCorrectName.ID, mock.MatchedBy(func(roleName gensql.RoleName) bool {
				return roleName != gensql.RoleNameTeamcreator
			})).
			Return(nil).
			Times(numDefaultRoleNames - 1)

		// user2@example.com
		dbtx.EXPECT().
			CreateUser(txCtx, "Create Me", "user2@example.com", "456").
			Return(createdLocalUser, nil).
			Once()
		dbtx.EXPECT().
			AssignGlobalRoleToUser(txCtx, createdLocalUser.ID, mock.AnythingOfType("gensql.RoleName")).
			Return(nil).
			Times(numDefaultRoleNames)

		// user3@example.com
		dbtx.EXPECT().
			UpdateUser(txCtx, localUserWithIncorrectEmail.ID, "Some Name", "user3@example.com", "789").
			Return(localUserWithCorrectEmail, nil).
			Once()
		dbtx.EXPECT().
			AssignGlobalRoleToUser(txCtx, localUserWithCorrectEmail.ID, mock.MatchedBy(func(roleName gensql.RoleName) bool {
				return roleName != gensql.RoleNameTeamviewer
			})).
			Return(nil).
			Times(numDefaultRoleNames - 1)

		dbtx.EXPECT().
			DeleteUser(txCtx, localUserThatWillBeDeleted.ID).
			Return(nil).
			Once()

		dbtx.EXPECT().
			AssignGlobalRoleToUser(txCtx, createdLocalUser.ID, gensql.RoleNameAdmin).
			Return(nil).
			Once()

		dbtx.EXPECT().
			RevokeGlobalUserRole(txCtx, localUserID1, gensql.RoleNameAdmin).
			Return(nil).
			Once()

		var actor *string
		db.EXPECT().
			CreateAuditLogEntry(
				ctx,
				mock.Anything,
				actor,
				audittype.AuditLogsTargetTypeUser,
				"user1@example.com",
				audittype.AuditActionUsersyncUpdate,
				`Local user updated: "user1@example.com", external ID: "123"`,
			).
			Return(nil).
			Once()
		db.EXPECT().
			CreateAuditLogEntry(
				ctx,
				mock.Anything,
				actor,
				audittype.AuditLogsTargetTypeUser,
				"user2@example.com",
				audittype.AuditActionUsersyncCreate,
				`Local user created: "user2@example.com", external ID: "456"`,
			).
			Return(nil).
			Once()
		db.EXPECT().
			CreateAuditLogEntry(
				ctx,
				mock.Anything,
				actor,
				audittype.AuditLogsTargetTypeUser,
				"user3@example.com",
				audittype.AuditActionUsersyncUpdate,
				`Local user updated: "user3@example.com", external ID: "789"`,
			).
			Return(nil).
			Once()

		db.EXPECT().
			CreateAuditLogEntry(
				ctx,
				mock.Anything,
				actor,
				audittype.AuditLogsTargetTypeUser,
				"delete-me@example.com",
				audittype.AuditActionUsersyncDelete,
				`Local user deleted: "delete-me@example.com", external ID: "321"`,
			).
			Return(nil).
			Once()

		db.EXPECT().
			CreateAuditLogEntry(
				ctx,
				mock.Anything,
				actor,
				audittype.AuditLogsTargetTypeUser,
				"user2@example.com",
				audittype.AuditActionUsersyncAssignAdminRole,
				`Assign global admin role to user: "user2@example.com"`,
			).
			Return(nil).
			Once()

		db.EXPECT().
			CreateAuditLogEntry(
				ctx,
				mock.Anything,
				actor,
				audittype.AuditLogsTargetTypeUser,
				"user1@example.com",
				audittype.AuditActionUsersyncRevokeAdminRole,
				`Revoke global admin role from user: "user1@example.com"`,
			).
			Return(nil).
			Once()

		err = usersync.
			New(db, auditLogger, adminGroupPrefix, domain, svc, log, syncRuns).
			Sync(ctx, correlationID)
		if err != nil {
			t.Fatal(err)
		}
	})
}
