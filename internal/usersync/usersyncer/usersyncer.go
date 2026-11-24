package usersyncer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/usersync/usersyncsql"
	"github.com/sirupsen/logrus"
	zitadeluser "github.com/zitadel/zitadel-go/v3/pkg/client/user/v2"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/object/v2"
	zitadelgrpcuser "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
	"go.opentelemetry.io/otel"
	admindirectoryv1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

type ZitadelWrapper struct {
	*zitadeluser.Client
	IDP                string
	OrganizationID     string
	NaisOrganizationID string
}

type Usersynchronizer struct {
	pool             *pgxpool.Pool
	querier          *usersyncsql.Queries
	adminGroupPrefix string
	userGroupPrefix  string
	tenantDomain     string
	service          *admindirectoryv1.Service
	log              logrus.FieldLogger
	zitadelClient    *ZitadelWrapper
	metrics          *syncMetrics
}

type userMap struct {
	byID         map[uuid.UUID]*usersyncsql.User
	byExternalID map[string]*usersyncsql.User
	byEmail      map[string]*usersyncsql.User
}

type sourceUser struct {
	ID           string
	Email        string
	Name         admindirectoryv1.UserName
	isTenantUser bool
}

type UserChanges struct {
	Created, Updated, Deleted int
}

type SyncResult struct {
	// Counts are nil if a complete snapshot was not available for the source.
	GoogleUsers, NaisUsers *int
	// Changes only counts committed database changes.
	Changes UserChanges
}

func New(pool *pgxpool.Pool, adminGroupPrefix, userGroupPrefix, tenantDomain string, zitadelWrapper *ZitadelWrapper, service *admindirectoryv1.Service, log logrus.FieldLogger) *Usersynchronizer {
	return &Usersynchronizer{
		pool:             pool,
		querier:          usersyncsql.New(pool),
		adminGroupPrefix: adminGroupPrefix,
		userGroupPrefix:  userGroupPrefix,
		tenantDomain:     tenantDomain,
		service:          service,
		log:              log,
		zitadelClient:    zitadelWrapper,
	}
}

func NewFromConfig(ctx context.Context, pool *pgxpool.Pool, serviceAccount, subjectEmail, tenantDomain, adminGroupPrefix, userGroupPrefix string, zitadelWrapper *ZitadelWrapper, log logrus.FieldLogger) (*Usersynchronizer, error) {
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

	metrics, err := newSyncMetrics(otel.Meter("nais_usersync"))
	if err != nil {
		return nil, fmt.Errorf("set up user sync metrics: %w", err)
	}
	s := New(pool, adminGroupPrefix, userGroupPrefix, tenantDomain, zitadelWrapper, srv, log)
	s.metrics = metrics
	return s, nil
}

// Sync fetches tenant users from Google Directory, optionally restricted to a group,
// and, when configured, active nais.io users from Zitadel. Both complete snapshots are fetched before changing
// the API database. Users are created or updated, and users absent from both
// sources are removed. Only tenant users are synchronized into Zitadel.
//
// Synced users present in the admin group in the Google Directory will also be granted the admin role in Nais API, and
// existing admins that no longer exist in the admin group will get the admin role revoked.
func (s *Usersynchronizer) Sync(ctx context.Context) (result SyncResult, err error) {
	var export sync.WaitGroup
	defer export.Wait()
	if s.metrics != nil {
		// Record the API run before waiting for the independent Zitadel export.
		start := time.Now()
		defer func() { s.metrics.recordRun(ctx, start, time.Now(), err == nil) }()
	}

	googleUsers, err := s.getGoogleUsers(ctx)
	if err != nil {
		return result, fmt.Errorf("get users from Google Directory: %w", err)
	}

	var naisUsers []*sourceUser
	importNaisUsers := s.zitadelClient != nil && s.zitadelClient.NaisOrganizationID != ""
	if importNaisUsers {
		naisUsers, err = s.zitadelClient.activeHumanUsers(ctx, s.zitadelClient.NaisOrganizationID)
		if err != nil {
			return result, fmt.Errorf("get nais.io users from Zitadel: %w", err)
		}
	}
	result.GoogleUsers = new(len(googleUsers))
	if importNaisUsers {
		result.NaisUsers = new(len(naisUsers))
	}
	if s.metrics != nil {
		s.metrics.recordSnapshot(ctx, "google", *result.GoogleUsers)
		if result.NaisUsers != nil {
			s.metrics.recordSnapshot(ctx, "nais_io", *result.NaisUsers)
		}
	}

	if s.zitadelClient != nil && s.zitadelClient.IDP != "" {
		export.Go(func() {
			success := s.zitadelUserSync(ctx, googleUsers)
			if s.metrics != nil {
				s.metrics.exportRuns.Add(ctx, 1, resultAttr(success))
			}
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("begin transaction: %w", err)
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
		return result, fmt.Errorf("get existing users: %w", err)
	}

	googleUserMap := make(map[string]*usersyncsql.User)
	var changes UserChanges
	for _, gu := range append(googleUsers, naisUsers...) {
		user, created, err := getOrCreateUser(ctx, querier, gu, users, s.log)
		if err != nil {
			return result, fmt.Errorf("get or create user %q: %w", gu.Email, err)
		}

		if created {
			changes.Created++
		}
		if userIsOutdated(user, gu) {
			if err := querier.Update(ctx, usersyncsql.UpdateParams{
				ID:         user.ID,
				Name:       gu.Name.FullName,
				Email:      gu.Email,
				ExternalID: gu.ID,
			}); err != nil {
				return result, fmt.Errorf("update user %q: %w", gu.Email, err)
			}
			changes.Updated++

			if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
				Action:       usersyncsql.UsersyncLogEntryActionUpdateUser,
				UserID:       user.ID,
				UserName:     gu.Name.FullName,
				UserEmail:    gu.Email,
				OldUserName:  &user.Name,
				OldUserEmail: &user.Email,
			}); err != nil {
				s.log.WithError(err).Errorf("create user sync log entry")
			}
		}

		if gu.isTenantUser {
			googleUserMap[gu.ID] = user
		}

		// Keep track of users absent from both source snapshots.
		delete(users.byID, user.ID)
	}

	if err := deleteUnknownUsers(ctx, querier, users.byID, s.log); err != nil {
		return result, err
	}
	changes.Deleted = len(users.byID)

	if err := assignAdmins(ctx, querier, s.service.Members, s.adminGroupPrefix, s.tenantDomain, googleUserMap, s.log); err != nil {
		return result, err
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	if s.metrics != nil {
		s.metrics.recordChanges(ctx, changes)
	}
	result.Changes = changes
	return result, nil
}

func (s *Usersynchronizer) zitadelUserSync(ctx context.Context, googleUsers []*sourceUser) bool {
	success := true
	start := time.Now()
	defer func() {
		s.log.WithField("duration", time.Since(start).String()).Debugf("Zitadel user sync done")
	}()

	limit, offset := uint32(250), uint64(0)
	existingUsers := make(map[string]*zitadelgrpcuser.User)

	for {
		resp, err := s.zitadelClient.ListUsers(ctx, &zitadelgrpcuser.ListUsersRequest{
			Query: &object.ListQuery{
				Offset: offset,
				Limit:  limit,
			},
			Queries: []*zitadelgrpcuser.SearchQuery{
				{
					Query: &zitadelgrpcuser.SearchQuery_TypeQuery{
						TypeQuery: &zitadelgrpcuser.TypeQuery{Type: zitadelgrpcuser.Type_TYPE_HUMAN},
					},
				},
				{
					Query: &zitadelgrpcuser.SearchQuery_OrganizationIdQuery{
						OrganizationIdQuery: &zitadelgrpcuser.OrganizationIdQuery{
							OrganizationId: s.zitadelClient.OrganizationID,
						},
					},
				},
				{
					Query: &zitadelgrpcuser.SearchQuery_StateQuery{
						StateQuery: &zitadelgrpcuser.StateQuery{State: zitadelgrpcuser.UserState_USER_STATE_ACTIVE},
					},
				},
			},
			SortingColumn: zitadelgrpcuser.UserFieldName_USER_FIELD_NAME_EMAIL,
		})
		if err != nil {
			s.log.WithError(err).Errorf("list users")
			return false
		}

		for _, user := range resp.Result {
			if user.GetHuman() == nil {
				s.log.WithField("user_id", user.UserId).Errorf("user is not human")
				success = false
				continue
			}

			existingUsers[user.UserId] = user
		}
		if len(resp.Result) < int(limit) {
			break
		}

		offset += uint64(limit)
	}
	for _, gu := range googleUsers {
		if u, exists := existingUsers[gu.ID]; exists {
			name := u.GetHuman().GetProfile().GetDisplayName()
			guName := gu.Name.GivenName + " " + gu.Name.FamilyName
			email := u.GetHuman().GetEmail().GetEmail()

			if name != guName || email != gu.Email {
				log := s.log.WithFields(logrus.Fields{
					"name":      guName,
					"email":     gu.Email,
					"old_name":  name,
					"old_email": email,
				})
				log.Debugf("update user in Zitadel")
				_, err := s.zitadelClient.UpdateHumanUser(ctx, &zitadelgrpcuser.UpdateHumanUserRequest{
					UserId:   gu.ID,
					Username: &gu.Email,
					Profile: &zitadelgrpcuser.SetHumanProfile{
						GivenName:   gu.Name.GivenName,
						FamilyName:  gu.Name.FamilyName,
						DisplayName: &guName,
					},
					Email: &zitadelgrpcuser.SetHumanEmail{
						Email: gu.Email,
						Verification: &zitadelgrpcuser.SetHumanEmail_IsVerified{
							IsVerified: true,
						},
					},
				})
				if err != nil {
					log.WithError(err).Errorf("update user in Zitadel")
					success = false
				}
			}

			delete(existingUsers, gu.ID)
			continue
		}

		log := s.log.WithFields(logrus.Fields{
			"given_name":  gu.Name.GivenName,
			"family_name": gu.Name.FamilyName,
			"email":       gu.Email,
		})
		log.Debugf("add user to Zitadel")
		_, err := s.zitadelClient.AddHumanUser(ctx, &zitadelgrpcuser.AddHumanUserRequest{
			UserId:   &gu.ID,
			Username: &gu.Email,
			Email: &zitadelgrpcuser.SetHumanEmail{
				Email: gu.Email,
				Verification: &zitadelgrpcuser.SetHumanEmail_IsVerified{
					IsVerified: true,
				},
			},
			Organization: &object.Organization{
				Org: &object.Organization_OrgDomain{
					OrgDomain: s.tenantDomain,
				},
			},
			Profile: &zitadelgrpcuser.SetHumanProfile{
				GivenName:   gu.Name.GivenName,
				FamilyName:  gu.Name.FamilyName,
				DisplayName: new(gu.Name.GivenName + " " + gu.Name.FamilyName),
			},
			IdpLinks: []*zitadelgrpcuser.IDPLink{
				{
					IdpId:    s.zitadelClient.IDP,
					UserId:   gu.ID,
					UserName: gu.Email,
				},
			},
		})
		if err != nil {
			log.WithError(err).Errorf("add user to Zitadel")
			success = false
			continue
		}
	}

	for userID, user := range existingUsers {
		log := s.log.WithFields(logrus.Fields{
			"user_id": userID,
			"name":    user.GetHuman().GetProfile().GetDisplayName(),
			"email":   user.GetHuman().GetEmail().GetEmail(),
		})
		log.Debugf("delete user from Zitadel")
		if _, err := s.zitadelClient.DeleteUser(ctx, &zitadelgrpcuser.DeleteUserRequest{UserId: userID}); err != nil {
			log.WithError(err).Errorf("delete user from Zitadel")
			success = false
		}
	}
	return success
}

func AssignDefaultPermissionsToUser(ctx context.Context, querier usersyncsql.Querier, userID uuid.UUID) error {
	defaultUserRoles := []string{
		"Team creator",
	}
	for _, roleName := range defaultUserRoles {
		if err := querier.AssignGlobalRole(ctx, usersyncsql.AssignGlobalRoleParams{
			UserID:   userID,
			RoleName: roleName,
		}); err != nil {
			return err
		}
	}
	return nil
}

// deleteUnknownUsers deletes API users absent from the complete source snapshots.
func deleteUnknownUsers(ctx context.Context, querier usersyncsql.Querier, unknownUsers map[uuid.UUID]*usersyncsql.User, log logrus.FieldLogger) error {
	for _, user := range unknownUsers {
		if err := querier.Delete(ctx, user.ID); err != nil {
			return fmt.Errorf("delete user %q: %w", user.Email, err)
		}
		if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
			Action:    usersyncsql.UsersyncLogEntryActionDeleteUser,
			UserID:    user.ID,
			UserName:  user.Name,
			UserEmail: user.Email,
		}); err != nil {
			log.WithError(err).Errorf("create user sync log entry")
		}
	}

	return nil
}

// assignAdmins assigns the global admin role to members of the admin group in the Google Directory of the tenant.
// Existing admins that is no longer a member of the admin group will have the admin role revoked.
func assignAdmins(ctx context.Context, querier usersyncsql.Querier, membersService *admindirectoryv1.MembersService, adminGroupPrefix, tenantDomain string, googleUsers map[string]*usersyncsql.User, log logrus.FieldLogger) error {
	admins, err := getAdminGroupMembers(ctx, membersService, adminGroupPrefix, tenantDomain, googleUsers, log)
	if err != nil {
		return err
	}

	existingAdmins, err := querier.ListGlobalAdmins(ctx)
	if err != nil {
		return err
	}

	for _, existingAdmin := range existingAdmins {
		if _, shouldBeAdmin := admins[existingAdmin.ID]; !shouldBeAdmin {
			log.WithField("email", existingAdmin.Email).Infof("revoke admin role")
			if err := querier.RevokeGlobalAdmin(ctx, existingAdmin.ID); err != nil {
				return err
			}

			if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
				Action:    usersyncsql.UsersyncLogEntryActionRevokeRole,
				UserID:    existingAdmin.ID,
				UserName:  existingAdmin.Name,
				UserEmail: existingAdmin.Email,
				RoleName:  new("Admin"),
			}); err != nil {
				log.WithError(err).Errorf("create user sync log entry")
			}
		}
	}

	for _, admin := range admins {
		if !admin.Admin {
			log.WithField("email", admin.Email).Infof("assign admin role")
			if err := querier.AssignGlobalAdmin(ctx, admin.ID); err != nil {
				return err
			}

			if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
				Action:    usersyncsql.UsersyncLogEntryActionAssignRole,
				UserID:    admin.ID,
				UserName:  admin.Name,
				UserEmail: admin.Email,
				RoleName:  new("Admin"),
			}); err != nil {
				log.WithError(err).Errorf("create user sync log entry")
			}
		}
	}

	return nil
}

// getAdminGroupMembers fetches all users in the admin group from the Google Directory of the tenant.
func getAdminGroupMembers(ctx context.Context, membersService *admindirectoryv1.MembersService, adminGroupPrefix, tenantDomain string, googleUsers map[string]*usersyncsql.User, log logrus.FieldLogger) (map[uuid.UUID]*usersyncsql.User, error) {
	adminGroup := adminGroupPrefix + "@" + tenantDomain
	admins := make(map[uuid.UUID]*usersyncsql.User)
	groupMembers, err := listActiveGroupUsers(ctx, membersService, adminGroup)
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
			log.WithField("email", member.Email).Debugf("admin group member is not a synced Google user")
			continue
		}

		admins[admin.ID] = admin
	}

	return admins, nil
}

func listActiveGroupUsers(ctx context.Context, svc *admindirectoryv1.MembersService, group string) ([]*admindirectoryv1.Member, error) {
	var members []*admindirectoryv1.Member
	err := svc.List(group).IncludeDerivedMembership(true).Pages(ctx, func(page *admindirectoryv1.Members) error {
		for _, member := range page.Members {
			if member.Type == "USER" && member.Status == "ACTIVE" {
				members = append(members, member)
			}
		}
		return nil
	})
	return members, err
}

// userIsOutdated checks if a user needs to get its name or its email address updated.
func userIsOutdated(user *usersyncsql.User, gu *sourceUser) bool {
	if user.Name != gu.Name.FullName {
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

// getOrCreateUser matches globally unique email addresses across user sources.
func getOrCreateUser(ctx context.Context, querier usersyncsql.Querier, remote *sourceUser, existingUsers *userMap, log logrus.FieldLogger) (*usersyncsql.User, bool, error) {
	if existingUser, exists := existingUsers.byEmail[remote.Email]; exists {
		return existingUser, false, nil
	}

	// Preserve the existing support for email changes without losing roles or sessions.
	if existingUser, exists := existingUsers.byExternalID[remote.ID]; exists {
		return existingUser, false, nil
	}

	createdUser, err := querier.Create(ctx, usersyncsql.CreateParams{
		Name:       remote.Name.FullName,
		Email:      remote.Email,
		ExternalID: remote.ID,
	})
	if err != nil {
		return nil, false, err
	}

	if remote.isTenantUser {
		if err := AssignDefaultPermissionsToUser(ctx, querier, createdUser.ID); err != nil {
			return nil, false, err
		}
	}

	if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
		Action:    usersyncsql.UsersyncLogEntryActionCreateUser,
		UserID:    createdUser.ID,
		UserName:  createdUser.Name,
		UserEmail: createdUser.Email,
	}); err != nil {
		log.WithError(err).Errorf("create user sync log entry")
	}

	return createdUser, true, nil
}

// getGoogleUsers fetches eligible tenant users from Google Directory.
func (s *Usersynchronizer) getGoogleUsers(ctx context.Context) ([]*sourceUser, error) {
	var allowedIDs map[string]struct{}
	if s.userGroupPrefix != "" {
		group := s.userGroupPrefix + "@" + s.tenantDomain
		members, err := listActiveGroupUsers(ctx, s.service.Members, group)
		if err != nil {
			// A failed lookup is not an empty snapshot: that would delete users.
			return nil, fmt.Errorf("list members in user sync group %q: %w", group, err)
		}
		allowedIDs = make(map[string]struct{}, len(members))
		for _, member := range members {
			allowedIDs[member.Id] = struct{}{}
		}
	}

	users := make([]*sourceUser, 0)
	callback := func(fragments *admindirectoryv1.Users) error {
		s.log.WithField("num", len(fragments.Users)).Debugf("fetched batch of users from Google Directory")
		for _, user := range fragments.Users {
			if _, allowed := allowedIDs[user.Id]; allowedIDs != nil && !allowed {
				continue
			}
			users = append(users, &sourceUser{
				ID:           user.Id,
				Email:        strings.ToLower(user.PrimaryEmail),
				Name:         *user.Name,
				isTenantUser: true,
			})
		}
		return nil
	}

	s.log.Debugf("start fetching users from Google Directory")
	t := time.Now()
	err := s.service.Users.
		List().
		Domain(s.tenantDomain).
		ShowDeleted("false").
		Query("isSuspended=false").
		Pages(ctx, callback)
	s.log.WithFields(logrus.Fields{
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
