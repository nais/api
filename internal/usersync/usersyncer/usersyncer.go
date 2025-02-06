package usersyncer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/usersync/usersyncsql"
	"github.com/sirupsen/logrus"
	admindirectoryv1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
	"k8s.io/utils/ptr"
)

type Usersynchronizer struct {
	pool             *pgxpool.Pool
	querier          *usersyncsql.Queries
	adminGroupPrefix string
	tenantDomain     string
	service          *admindirectoryv1.Service
	log              logrus.FieldLogger
}

type userMap struct {
	byID         map[uuid.UUID]*usersyncsql.User
	byExternalID map[string]*usersyncsql.User
	byEmail      map[string]*usersyncsql.User
}

type userRolesMap map[*usersyncsql.User]map[string]struct{}

type googleUser struct {
	ID    string
	Email string
	Name  string
}

// DefaultRoleNames are the default set of roles that will be assigned to all new users.
var DefaultRoleNames = []string{
	"Team creator",
	"Team viewer",
	"User viewer",
}

func New(pool *pgxpool.Pool, adminGroupPrefix, tenantDomain string, service *admindirectoryv1.Service, log logrus.FieldLogger) *Usersynchronizer {
	return &Usersynchronizer{
		pool:             pool,
		querier:          usersyncsql.New(pool),
		adminGroupPrefix: adminGroupPrefix,
		tenantDomain:     tenantDomain,
		service:          service,
		log:              log,
	}
}

func NewFromConfig(ctx context.Context, pool *pgxpool.Pool, serviceAccount, subjectEmail, tenantDomain, adminGroupPrefix string, log logrus.FieldLogger) (*Usersynchronizer, error) {
	ts, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
		Scopes: []string{
			admindirectoryv1.AdminDirectoryUserReadonlyScope,
			admindirectoryv1.AdminDirectoryGroupScope,
		},
		Subject:         subjectEmail,
		TargetPrincipal: serviceAccount,
	})
	if err != nil {
		return nil, fmt.Errorf("create token source: %w", err)
	}

	srv, err := admindirectoryv1.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("create admin directory client: %w", err)
	}

	return New(pool, adminGroupPrefix, tenantDomain, srv, log), nil
}

// Sync fetches all users from the Google Directory of the tenant and adds them as users in NAIS API.
//
// If a user already exist in NAIS API the user will get the name and email potentially updated if it has changed in the
// Google Directory.
//
// After all users have been synced, users that have an email address that matches the tenant domain that no longer
// exist in the Google Directory will be removed.
//
// All users present in the admin group in the Google Directory will also be granted the admin role in NAIS API, and
// existing admins that no longer exist in the admin group will get the admin role revoked.
func (s *Usersynchronizer) Sync(ctx context.Context) error {
	googleUsers, err := getGoogleUsers(ctx, s.service.Users, s.tenantDomain, s.log)
	if err != nil {
		return fmt.Errorf("get users from Google Directory: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err == nil {
			return
		} else if !errors.Is(err, pgx.ErrTxClosed) {
			s.log.WithError(err).Errorf("rollback transaction")
		}
	}()
	querier := s.querier.WithTx(tx)

	users, err := getUsers(ctx, querier)
	if err != nil {
		return fmt.Errorf("get existing users: %w", err)
	}

	userRoles, err := getUserRoles(ctx, querier, users)
	if err != nil {
		return fmt.Errorf("get existing user roles: %w", err)
	}

	googleUserMap := make(map[string]*usersyncsql.User)
	for _, gu := range googleUsers {
		user, err := getOrCreateUserFromGoogleUser(ctx, querier, gu, users, s.log)
		if err != nil {
			return fmt.Errorf("get or create user %q: %w", gu.Email, err)
		}

		if userIsOutdated(user, gu) {
			if err := querier.Update(ctx, usersyncsql.UpdateParams{
				ID:         user.ID,
				Name:       gu.Name,
				Email:      gu.Email,
				ExternalID: gu.ID,
			}); err != nil {
				return fmt.Errorf("update user %q: %w", gu.Email, err)
			}

			if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
				Action:       usersyncsql.UsersyncLogEntryActionUpdateUser,
				UserID:       user.ID,
				UserName:     gu.Name,
				UserEmail:    gu.Email,
				OldUserName:  &user.Name,
				OldUserEmail: &user.Email,
			}); err != nil {
				s.log.WithError(err).Errorf("create user sync log entry")
			}
		}

		for _, roleName := range DefaultRoleNames {
			if globalRoles, userHasGlobalRoles := userRoles[user]; userHasGlobalRoles {
				if _, userHasDefaultRole := globalRoles[roleName]; userHasDefaultRole {
					continue
				}
			}
			if err := querier.AssignGlobalRole(ctx, usersyncsql.AssignGlobalRoleParams{
				UserID:   user.ID,
				RoleName: roleName,
			}); err != nil {
				return fmt.Errorf("attach default role %q to user %q: %w", roleName, user.Email, err)
			}

			if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
				Action:    usersyncsql.UsersyncLogEntryActionAssignRole,
				UserID:    user.ID,
				UserName:  user.Name,
				UserEmail: user.Email,
				RoleName:  ptr.To(string(roleName)),
			}); err != nil {
				s.log.WithError(err).Errorf("create user sync log entry")
			}
		}

		googleUserMap[gu.ID] = user

		// remove user from map to keep track of users that no longer exist in the Google Directory
		delete(users.byID, user.ID)
	}

	deletedUsers, err := deleteUnknownUsers(ctx, querier, users.byID, s.log)
	if err != nil {
		return err
	}

	for _, deletedUser := range deletedUsers {
		delete(userRoles, deletedUser)
	}

	if err := assignAdmins(ctx, querier, s.service.Members, s.adminGroupPrefix, s.tenantDomain, googleUserMap, userRoles, s.log); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// deleteUnknownUsers will delete users from NAIS API that does not exist in the Google Directory.
func deleteUnknownUsers(ctx context.Context, querier usersyncsql.Querier, unknownUsers map[uuid.UUID]*usersyncsql.User, log logrus.FieldLogger) ([]*usersyncsql.User, error) {
	ret := make([]*usersyncsql.User, 0)
	for _, user := range unknownUsers {
		if err := querier.Delete(ctx, user.ID); err != nil {
			return nil, fmt.Errorf("delete user %q: %w", user.Email, err)
		}
		ret = append(ret, user)
		if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
			Action:    usersyncsql.UsersyncLogEntryActionDeleteUser,
			UserID:    user.ID,
			UserName:  user.Name,
			UserEmail: user.Email,
		}); err != nil {
			log.WithError(err).Errorf("create user sync log entry")
		}
	}

	return ret, nil
}

// assignAdmins assigns the global admin role to members of the admin group in the Google Directory of the tenant.
// Existing admins that is no longer a member of the admin group will have the admin role revoked.
func assignAdmins(ctx context.Context, querier usersyncsql.Querier, membersService *admindirectoryv1.MembersService, adminGroupPrefix, tenantDomain string, googleUsers map[string]*usersyncsql.User, userRoles userRolesMap, log logrus.FieldLogger) error {
	admins, err := getAdminGroupMembers(ctx, membersService, adminGroupPrefix, tenantDomain, googleUsers, log)
	if err != nil {
		return err
	}

	existingAdmins := getExistingAdmins(userRoles)
	for _, existingAdmin := range existingAdmins {
		if _, shouldBeAdmin := admins[existingAdmin.ID]; !shouldBeAdmin {
			log.WithField("email", existingAdmin.Email).Debugf("revoke admin role")
			if err := querier.RevokeGlobalRole(ctx, usersyncsql.RevokeGlobalRoleParams{
				UserID:   existingAdmin.ID,
				RoleName: "Admin",
			}); err != nil {
				return err
			}

			if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
				Action:    usersyncsql.UsersyncLogEntryActionRevokeRole,
				UserID:    existingAdmin.ID,
				UserName:  existingAdmin.Name,
				UserEmail: existingAdmin.Email,
				RoleName:  ptr.To("Admin"),
			}); err != nil {
				log.WithError(err).Errorf("create user sync log entry")
			}
		}
	}

	for _, admin := range admins {
		if _, isAlreadyAdmin := existingAdmins[admin.ID]; !isAlreadyAdmin {
			log.WithField("email", admin.Email).Debugf("assign admin role")
			if err := querier.AssignGlobalRole(ctx, usersyncsql.AssignGlobalRoleParams{
				UserID:   admin.ID,
				RoleName: "Admin",
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// getExistingAdmins returns all users with a globally assigned admin role.
func getExistingAdmins(userWithRoles userRolesMap) map[uuid.UUID]*usersyncsql.User {
	admins := make(map[uuid.UUID]*usersyncsql.User)
	for user, roles := range userWithRoles {
		for roleName := range roles {
			if roleName == "Admin" {
				admins[user.ID] = user
			}
		}
	}
	return admins
}

// getAdminGroupMembers fetches all users in the admin group from the Google Directory of the tenant.
func getAdminGroupMembers(ctx context.Context, membersService *admindirectoryv1.MembersService, adminGroupPrefix, tenantDomain string, googleUsers map[string]*usersyncsql.User, log logrus.FieldLogger) (map[uuid.UUID]*usersyncsql.User, error) {
	adminGroup := adminGroupPrefix + "@" + tenantDomain
	groupMembers := make([]*admindirectoryv1.Member, 0)
	callback := func(fragments *admindirectoryv1.Members) error {
		for _, member := range fragments.Members {
			if member.Type == "USER" && member.Status == "ACTIVE" {
				groupMembers = append(groupMembers, member)
			}
		}
		return nil
	}
	admins := make(map[uuid.UUID]*usersyncsql.User)
	err := membersService.
		List(adminGroup).
		IncludeDerivedMembership(true).
		Pages(ctx, callback)
	if err != nil {
		if googleError, ok := err.(*googleapi.Error); ok && googleError.Code == 404 {
			// Special case: When the group does not exist we want to remove all existing admins. The group might have
			// never been created by the tenant admins in the first place, or it might have been deleted. In any case,
			// we want to treat this case as if the group exists, and that it is empty, effectively removing all admins.
			log.WithField("group_name", adminGroup).Warnf("api admins group does not exist")
			return admins, nil
		}

		return nil, fmt.Errorf("list members in api admins group: %w", err)
	}

	for _, member := range groupMembers {
		admin, exists := googleUsers[member.Id]
		if !exists {
			log.WithField("email", member.Email).Errorf("unknown user in admins groups")
			continue
		}

		admins[admin.ID] = admin
	}

	return admins, nil
}

// userIsOutdated checks if a user needs to get its name or its email address updated.
func userIsOutdated(user *usersyncsql.User, gu *googleUser) bool {
	if user.Name != gu.Name {
		return true
	}

	if !strings.EqualFold(user.Email, gu.Email) {
		return true
	}

	if user.ExternalID != gu.ID {
		return true
	}

	return false
}

// getOrCreateUserFromGoogleUser will return a user for a Google user, creating it first if needed.
func getOrCreateUserFromGoogleUser(ctx context.Context, querier usersyncsql.Querier, googleUser *googleUser, existingUsers *userMap, log logrus.FieldLogger) (*usersyncsql.User, error) {
	if existingUser, exists := existingUsers.byExternalID[googleUser.ID]; exists {
		return existingUser, nil
	}

	if existingUser, exists := existingUsers.byEmail[googleUser.Email]; exists {
		return existingUser, nil
	}

	createdUser, err := querier.Create(ctx, usersyncsql.CreateParams{
		Name:       googleUser.Name,
		Email:      googleUser.Email,
		ExternalID: googleUser.ID,
	})
	if err != nil {
		return nil, err
	}

	if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
		Action:    usersyncsql.UsersyncLogEntryActionCreateUser,
		UserID:    createdUser.ID,
		UserName:  createdUser.Name,
		UserEmail: createdUser.Email,
	}); err != nil {
		log.WithError(err).Errorf("create user sync log entry")
	}

	return createdUser, nil
}

// getGoogleUsers fetches all users from the Google Directory.
func getGoogleUsers(ctx context.Context, svc *admindirectoryv1.UsersService, tenantDomain string, log logrus.FieldLogger) ([]*googleUser, error) {
	users := make([]*googleUser, 0)
	callback := func(fragments *admindirectoryv1.Users) error {
		log.WithField("num", len(fragments.Users)).Debugf("fetched batch of users from from Google Directory")
		for _, user := range fragments.Users {
			users = append(users, &googleUser{
				ID:    user.Id,
				Email: strings.ToLower(user.PrimaryEmail),
				Name:  user.Name.FullName,
			})
		}
		return nil
	}

	log.Debugf("start fetching users from Google Directory")
	t := time.Now()
	err := svc.
		List().
		Domain(tenantDomain).
		ShowDeleted("false").
		Query("isSuspended=false").
		Pages(ctx, callback)
	log.WithFields(logrus.Fields{
		"duration":  time.Since(t),
		"num_users": len(users),
	}).Debugf("finished fetching users from Google Directory")
	return users, err
}

// getUsers return a collection of maps of users by ID, external ID and email.
func getUsers(ctx context.Context, querier usersyncsql.Querier) (*userMap, error) {
	users, err := querier.List(ctx)
	if err != nil {
		return nil, err
	}
	ret := &userMap{
		byID:         make(map[uuid.UUID]*usersyncsql.User),
		byExternalID: make(map[string]*usersyncsql.User),
		byEmail:      make(map[string]*usersyncsql.User),
	}
	for _, user := range users {
		ret.byID[user.ID] = user
		ret.byExternalID[user.ExternalID] = user
		ret.byEmail[user.Email] = user
	}

	return ret, nil
}

// getUserRoles returns a map of users and their roles.
func getUserRoles(ctx context.Context, querier usersyncsql.Querier, users *userMap) (userRolesMap, error) {
	roles, err := querier.ListRoles(ctx)
	if err != nil {
		return nil, err
	}

	userRoles := make(userRolesMap)
	for _, role := range roles {
		user := users.byID[role.UserID]
		if _, exists := userRoles[user]; !exists {
			userRoles[user] = make(map[string]struct{})
		}
		userRoles[user][role.RoleName] = struct{}{}
	}

	return userRoles, nil
}
