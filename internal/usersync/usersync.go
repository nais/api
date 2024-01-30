package usersync

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/thirdparty/google_token_source"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	admin_directory_v1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type (
	UserSynchronizer struct {
		database         database.Database
		auditLogger      auditlogger.AuditLogger
		adminGroupPrefix string
		tenantDomain     string
		service          *admin_directory_v1.Service
		log              logrus.FieldLogger
		syncRuns         *RunsHandler
	}

	auditLogEntry struct {
		action    audittype.AuditAction
		userEmail string
		message   string
	}

	// Key is the ID from Azure AD
	remoteUsersMap map[string]*database.User

	userMap struct {
		// byExternalID key is the ID from Azure AD
		byExternalID map[string]*database.User
		byEmail      map[string]*database.User
	}

	userByIDMap  map[uuid.UUID]*database.User
	userRolesMap map[*database.User]map[gensql.RoleName]struct{}
)

var DefaultRoleNames = []gensql.RoleName{
	gensql.RoleNameTeamcreator,
	gensql.RoleNameTeamviewer,
	gensql.RoleNameUserviewer,
	gensql.RoleNameServiceaccountcreator,
}

func New(dbc database.Database, auditLogger auditlogger.AuditLogger, adminGroupPrefix, tenantDomain string, service *admin_directory_v1.Service, log logrus.FieldLogger, syncRuns *RunsHandler) *UserSynchronizer {
	return &UserSynchronizer{
		database:         dbc,
		auditLogger:      auditLogger,
		adminGroupPrefix: adminGroupPrefix,
		tenantDomain:     tenantDomain,
		service:          service,
		log:              log,
		syncRuns:         syncRuns,
	}
}

func NewFromConfig(googleManagementProjectID, tenantDomain, adminGroupPrefix string, db database.Database, log logrus.FieldLogger, syncRuns *RunsHandler) (*UserSynchronizer, error) {
	log = log.WithField("component", logger.ComponentNameUsersync)
	ctx := context.Background()

	builder, err := google_token_source.New(googleManagementProjectID, tenantDomain)
	if err != nil {
		return nil, err
	}

	ts, err := builder.Admin(ctx)
	if err != nil {
		return nil, fmt.Errorf("create token source: %w", err)
	}

	srv, err := admin_directory_v1.NewService(ctx, option.WithTokenSource(ts), option.WithHTTPClient(&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}))
	if err != nil {
		return nil, fmt.Errorf("retrieve directory client: %w", err)
	}

	return New(db, auditlogger.New(db, logger.ComponentNameUsersync, log), adminGroupPrefix, tenantDomain, srv, log, syncRuns), nil
}

// Sync Fetch all users from the tenant and add them as local users in api. If a user already exists in
// api the local user will get the name potentially updated. After all users have been upserted, local users
// that matches the tenant domain that does not exist in the Google Directory will be removed.
func (s *UserSynchronizer) Sync(ctx context.Context, correlationID uuid.UUID) error {
	log := s.log.WithField("correlation_id", correlationID)
	syncRun := s.syncRuns.StartNewRun(correlationID)
	defer syncRun.Finish()

	remoteUserMapping := make(remoteUsersMap)
	remoteUsers, err := getAllPaginatedUsers(ctx, s.service.Users, s.tenantDomain)
	if err != nil {
		syncRun.FinishWithError(err)
		return fmt.Errorf("get remote users: %w", err)
	}

	auditLogEntries := make([]auditLogEntry, 0)
	err = s.database.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		allUsersRows, err := dbtx.GetAllUsers(ctx)
		if err != nil {
			return fmt.Errorf("get existing users: %w", err)
		}

		usersByID := make(userByIDMap)
		existingUsers := userMap{
			byExternalID: make(map[string]*database.User),
			byEmail:      make(map[string]*database.User),
		}

		for _, user := range allUsersRows {
			usersByID[user.ID] = user
			existingUsers.byExternalID[user.ExternalID] = user
			existingUsers.byEmail[user.Email] = user
		}

		allUserRolesRows, err := dbtx.GetAllUserRoles(ctx)
		if err != nil {
			return fmt.Errorf("get existing user roles: %w", err)
		}

		userRoles := make(userRolesMap)
		for _, row := range allUserRolesRows {
			user := usersByID[row.UserID]
			if _, exists := userRoles[user]; !exists {
				userRoles[user] = make(map[gensql.RoleName]struct{})
			}
			userRoles[user][row.RoleName] = struct{}{}
		}

		for _, remoteUser := range remoteUsers {
			email := strings.ToLower(remoteUser.PrimaryEmail)
			localUser, created, err := getOrCreateLocalUserFromRemoteUser(ctx, dbtx, remoteUser, existingUsers)
			if err != nil {
				return fmt.Errorf("get or create local user %q: %w", email, err)
			}

			if created {
				auditLogEntries = append(auditLogEntries, auditLogEntry{
					action:    audittype.AuditActionUsersyncCreate,
					message:   fmt.Sprintf("Local user created: %q, external ID: %q", localUser.Email, localUser.ExternalID),
					userEmail: localUser.Email,
				})
			}

			if localUserIsOutdated(localUser, remoteUser) {
				updatedUser, err := dbtx.UpdateUser(ctx, localUser.ID, remoteUser.Name.FullName, email, remoteUser.Id)
				if err != nil {
					return fmt.Errorf("update local user %q: %w", email, err)
				}

				auditLogEntries = append(auditLogEntries, auditLogEntry{
					action:    audittype.AuditActionUsersyncUpdate,
					message:   fmt.Sprintf("Local user updated: %q, external ID: %q", updatedUser.Email, updatedUser.ExternalID),
					userEmail: updatedUser.Email,
				})
			}

			for _, roleName := range DefaultRoleNames {
				if globalRoles, userHasGlobalRoles := userRoles[localUser]; userHasGlobalRoles {
					if _, userHasDefaultRole := globalRoles[roleName]; userHasDefaultRole {
						continue
					}
				}
				err = dbtx.AssignGlobalRoleToUser(ctx, localUser.ID, roleName)
				if err != nil {
					return fmt.Errorf("attach default role %q to user %q: %w", roleName, email, err)
				}
			}

			remoteUserMapping[remoteUser.Id] = localUser
			delete(usersByID, localUser.ID)
		}

		deletedUsers, err := deleteUnknownUsers(ctx, dbtx, usersByID, &auditLogEntries)
		if err != nil {
			return err
		}

		for _, deletedUser := range deletedUsers {
			delete(userRoles, deletedUser)
		}

		err = assignTeamsBackendAdmins(ctx, dbtx, s.service.Members, s.adminGroupPrefix, s.tenantDomain, remoteUserMapping, userRoles, &auditLogEntries, log)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		syncRun.FinishWithError(err)
		return err
	}

	for _, entry := range auditLogEntries {
		targets := []auditlogger.Target{
			auditlogger.UserTarget(entry.userEmail),
		}
		fields := auditlogger.Fields{
			Action:        entry.action,
			CorrelationID: correlationID,
		}
		s.auditLogger.Logf(ctx, targets, fields, entry.message)
	}

	return nil
}

// deleteUnknownUsers Delete users from the api database that does not exist in the Google Workspace
func deleteUnknownUsers(ctx context.Context, dbtx database.Database, unknownUsers userByIDMap, auditLogEntries *[]auditLogEntry) ([]*database.User, error) {
	deletedUsers := make([]*database.User, 0)
	for _, user := range unknownUsers {
		err := dbtx.DeleteUser(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("delete local user %q: %w", user.Email, err)
		}
		*auditLogEntries = append(*auditLogEntries, auditLogEntry{
			action:    audittype.AuditActionUsersyncDelete,
			message:   fmt.Sprintf("Local user deleted: %q, external ID: %q", user.Email, user.ExternalID),
			userEmail: user.Email,
		})
		deletedUsers = append(deletedUsers, user)
	}

	return deletedUsers, nil
}

// assignTeamsBackendAdmins Assign the global admin role to users based on the admin group. Existing admins that is not
// present in the list of admins will get the admin role revoked.
func assignTeamsBackendAdmins(ctx context.Context, dbtx database.Database, membersService *admin_directory_v1.MembersService, adminGroupPrefix, tenantDomain string, remoteUserMapping map[string]*database.User, userRoles userRolesMap, auditLogEntries *[]auditLogEntry, log logrus.FieldLogger) error {
	admins, err := getAdminUsers(ctx, membersService, adminGroupPrefix, tenantDomain, remoteUserMapping, log)
	if err != nil {
		return err
	}

	existingAdmins := getExistingTeamsBackendAdmins(userRoles)
	for _, existingAdmin := range existingAdmins {
		if _, shouldBeAdmin := admins[existingAdmin.ID]; !shouldBeAdmin {
			err = dbtx.RevokeGlobalUserRole(ctx, existingAdmin.ID, gensql.RoleNameAdmin)
			if err != nil {
				return err
			}

			*auditLogEntries = append(*auditLogEntries, auditLogEntry{
				action:    audittype.AuditActionUsersyncRevokeAdminRole,
				message:   fmt.Sprintf("Revoke global admin role from user: %q", existingAdmin.Email),
				userEmail: existingAdmin.Email,
			})
		}
	}

	for _, admin := range admins {
		if _, isAlreadyAdmin := existingAdmins[admin.ID]; !isAlreadyAdmin {
			err = dbtx.AssignGlobalRoleToUser(ctx, admin.ID, gensql.RoleNameAdmin)
			if err != nil {
				return err
			}

			*auditLogEntries = append(*auditLogEntries, auditLogEntry{
				action:    audittype.AuditActionUsersyncAssignAdminRole,
				message:   fmt.Sprintf("Assign global admin role to user: %q", admin.Email),
				userEmail: admin.Email,
			})
		}
	}

	return nil
}

// getExistingTeamsBackendAdmins Get all users with a globally assigned admin role
func getExistingTeamsBackendAdmins(userWithRoles userRolesMap) map[uuid.UUID]*database.User {
	admins := make(map[uuid.UUID]*database.User)
	for user, roles := range userWithRoles {
		for roleName := range roles {
			if roleName == gensql.RoleNameAdmin {
				admins[user.ID] = user
			}
		}
	}
	return admins
}

// getAdminUsers Get a list of admin users based on the api admins group in the Google Workspace
func getAdminUsers(ctx context.Context, membersService *admin_directory_v1.MembersService, adminGroupPrefix, tenantDomain string, remoteUserMapping map[string]*database.User, log logrus.FieldLogger) (map[uuid.UUID]*database.User, error) {
	adminGroupKey := adminGroupPrefix + "@" + tenantDomain
	groupMembers := make([]*admin_directory_v1.Member, 0)
	callback := func(fragments *admin_directory_v1.Members) error {
		for _, member := range fragments.Members {
			if member.Type == "USER" && member.Status == "ACTIVE" {
				groupMembers = append(groupMembers, member)
			}
		}
		return nil
	}
	admins := make(map[uuid.UUID]*database.User)
	err := membersService.
		List(adminGroupKey).
		IncludeDerivedMembership(true).
		Context(ctx).
		Pages(ctx, callback)
	if err != nil {
		if googleError, ok := err.(*googleapi.Error); ok && googleError.Code == 404 {
			// Special case: When the group does not exist we want to remove all existing admins. The group might never
			// have been created by the tenant admins in the first place, or it might have been deleted. In any case, we
			// want to treat this case as if the group exists, and that it is empty, effectively removing all admins.
			log.Warnf("api admins group %q does not exist", adminGroupKey)
			return admins, nil
		}

		return nil, fmt.Errorf("list members in api admins group: %w", err)
	}

	for _, member := range groupMembers {
		admin, exists := remoteUserMapping[member.Id]
		if !exists {
			log.Errorf("unknown user %q in admins groups", member.Email)
			continue
		}

		admins[admin.ID] = admin
	}

	return admins, nil
}

// localUserIsOutdated Check if a local user is outdated when compared to the remote user
func localUserIsOutdated(localUser *database.User, remoteUser *admin_directory_v1.User) bool {
	if localUser.Name != remoteUser.Name.FullName {
		return true
	}

	if !strings.EqualFold(localUser.Email, remoteUser.PrimaryEmail) {
		return true
	}

	if localUser.ExternalID != remoteUser.Id {
		return true
	}

	return false
}

// getOrCreateLocalUserFromRemoteUser Look up the local user table for a match for the remote user. If no match is
// found, create the user.
func getOrCreateLocalUserFromRemoteUser(ctx context.Context, dbtx database.Database, remoteUser *admin_directory_v1.User, existingUsers userMap) (*database.User, bool, error) {
	if existingUser, exists := existingUsers.byExternalID[remoteUser.Id]; exists {
		return existingUser, false, nil
	}

	email := strings.ToLower(remoteUser.PrimaryEmail)
	if existingUser, exists := existingUsers.byEmail[email]; exists {
		return existingUser, false, nil
	}

	createdUser, err := dbtx.CreateUser(ctx, remoteUser.Name.FullName, email, remoteUser.Id)
	if err != nil {
		return nil, false, err
	}

	return createdUser, true, nil
}

func getAllPaginatedUsers(ctx context.Context, svc *admin_directory_v1.UsersService, tenantDomain string) ([]*admin_directory_v1.User, error) {
	users := make([]*admin_directory_v1.User, 0)

	callback := func(fragments *admin_directory_v1.Users) error {
		users = append(users, fragments.Users...)
		return nil
	}

	err := svc.
		List().
		Context(ctx).
		Domain(tenantDomain).
		ShowDeleted("false").
		Query("isSuspended=false").
		Pages(ctx, callback)

	return users, err
}
