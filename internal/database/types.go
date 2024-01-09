package database

import (
	"context"
	"time"

	"github.com/nais/api/internal/types"

	"github.com/nais/api/internal/roles"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

const teamDeleteKeyLifetime = time.Hour * 1

type (
	QuerierTransactionFunc  func(ctx context.Context, querier Querier) error
	DatabaseTransactionFunc func(ctx context.Context, dbtx Database) error
)

type AuthenticatedUser interface {
	GetID() uuid.UUID
	Identity() string
	IsServiceAccount() bool
}

type AuditLog struct {
	*sqlc.AuditLog
}

type Reconciler struct {
	*sqlc.Reconciler
}

type TeamDeleteKey struct {
	*sqlc.TeamDeleteKey
}

type ReconcilerConfig struct {
	*sqlc.GetReconcilerConfigRow
}

type ReconcilerConfigValues struct {
	values map[sqlc.ReconcilerConfigKey]string
}

func (v ReconcilerConfigValues) GetValue(s sqlc.ReconcilerConfigKey) string {
	if v, exists := v.values[s]; exists {
		return v
	}
	return ""
}

type ReconcilerError struct {
	*sqlc.ReconcilerError
}

type UserRole struct {
	*sqlc.UserRole
}

type Role struct {
	Authorizations         []roles.Authorization
	RoleName               sqlc.RoleName
	TargetServiceAccountID *uuid.UUID
	TargetTeamSlug         *slug.Slug
}

type ServiceAccount struct {
	*sqlc.ServiceAccount
}

type Session struct {
	*sqlc.Session
}

type Team struct {
	*sqlc.Team
}

type UserTeam struct {
	*sqlc.Team
	RoleName sqlc.RoleName
}

func (t *Team) IsEntity() {}

type User struct {
	*sqlc.User
	IsAdmin *bool
}

func (u *User) IsEntity() {}

type Querier interface {
	sqlc.Querier
	Transaction(ctx context.Context, callback QuerierTransactionFunc) error
}

type Queries struct {
	*sqlc.Queries
	connPool *pgxpool.Pool
}

type database struct {
	querier Querier
}

type Database interface {
	CreateRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization sqlc.RepositoryAuthorizationEnum) error
	RemoveRepositoryAuthorization(ctx context.Context, teamSlug slug.Slug, repoName string, authorization sqlc.RepositoryAuthorizationEnum) error
	CreateAuditLogEntry(ctx context.Context, correlationID uuid.UUID, componentName types.ComponentName, actor *string, targetType types.AuditLogsTargetType, targetIdentifier string, action types.AuditAction, message string) error
	CreateUser(ctx context.Context, name, email, externalID string) (*User, error)
	CreateServiceAccount(ctx context.Context, name string) (*ServiceAccount, error)
	GetServiceAccountByName(ctx context.Context, name string) (*ServiceAccount, error)
	GetUserByID(ctx context.Context, ID uuid.UUID) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetServiceAccountByApiKey(ctx context.Context, APIKey string) (*ServiceAccount, error)
	DeleteUser(ctx context.Context, userID uuid.UUID) error
	GetUsers(ctx context.Context, offset, limit int) ([]*User, int, error)
	GetAllUsers(ctx context.Context) ([]*User, error)
	GetUserTeams(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*UserTeam, int, error)
	CreateTeam(ctx context.Context, slug slug.Slug, purpose, slackChannel string) (*Team, error)
	UpdateTeam(ctx context.Context, teamSlug slug.Slug, purpose, slackChannel *string) (*Team, error)
	GetActiveTeamBySlug(ctx context.Context, slug slug.Slug) (*Team, error)
	GetTeamBySlug(ctx context.Context, slug slug.Slug) (*Team, error)
	GetActiveTeams(ctx context.Context) ([]*Team, error)
	GetTeams(ctx context.Context, offset, limit int) ([]*Team, int, error)
	GetAllTeams(ctx context.Context) ([]*Team, error)
	GetTeamMembers(ctx context.Context, teamSlug slug.Slug, offset, limit int) ([]*User, int, error)
	GetAllTeamMembers(ctx context.Context, teamSlug slug.Slug) ([]*User, error)
	GetTeamMember(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*User, error)
	UserIsTeamOwner(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) (bool, error)
	SetTeamMemberRole(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, role sqlc.RoleName) error
	GetAuditLogsForTeam(ctx context.Context, slug slug.Slug, offset, limit int) ([]*AuditLog, int, error)
	AssignGlobalRoleToUser(ctx context.Context, userID uuid.UUID, roleName sqlc.RoleName) error
	AssignGlobalRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName sqlc.RoleName) error
	AssignTeamRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName sqlc.RoleName, teamSlug slug.Slug) error
	RemoveUserFromTeam(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error
	CreateAPIKey(ctx context.Context, apiKey string, serviceAccountID uuid.UUID) error
	RemoveAllServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) error
	RemoveApiKeysFromServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*Role, error)
	GetAllUserRoles(ctx context.Context) ([]*UserRole, error)
	GetServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) ([]*Role, error)
	Transaction(ctx context.Context, fn DatabaseTransactionFunc) error
	LoadReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug, state interface{}) error
	SetReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug, state interface{}) error
	RemoveReconcilerStateForTeam(ctx context.Context, reconcilerName sqlc.ReconcilerName, slug slug.Slug) error
	UpdateUser(ctx context.Context, userID uuid.UUID, name, email, externalID string) (*User, error)
	SetReconcilerErrorForTeam(ctx context.Context, correlationID uuid.UUID, slug slug.Slug, reconcilerName sqlc.ReconcilerName, err error) error
	GetTeamReconcilerErrors(ctx context.Context, slug slug.Slug) ([]*ReconcilerError, error)
	ClearReconcilerErrorsForTeam(ctx context.Context, slug slug.Slug, reconcilerName sqlc.ReconcilerName) error
	GetServiceAccounts(ctx context.Context) ([]*ServiceAccount, error)
	DeleteServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error
	GetUserByExternalID(ctx context.Context, externalID string) (*User, error)
	CreateSession(ctx context.Context, userID uuid.UUID) (*Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*Session, error)
	ExtendSession(ctx context.Context, sessionID uuid.UUID) (*Session, error)
	GetReconciler(ctx context.Context, reconcilerName sqlc.ReconcilerName) (*Reconciler, error)
	GetReconcilers(ctx context.Context) ([]*Reconciler, error)
	GetEnabledReconcilers(ctx context.Context) ([]*Reconciler, error)
	ConfigureReconciler(ctx context.Context, reconcilerName sqlc.ReconcilerName, key sqlc.ReconcilerConfigKey, value string) error
	GetReconcilerConfig(ctx context.Context, reconcilerName sqlc.ReconcilerName) ([]*ReconcilerConfig, error)
	ResetReconcilerConfig(ctx context.Context, reconcilerName sqlc.ReconcilerName) (*Reconciler, error)
	EnableReconciler(ctx context.Context, reconcilerName sqlc.ReconcilerName) (*Reconciler, error)
	DisableReconciler(ctx context.Context, reconcilerName sqlc.ReconcilerName) (*Reconciler, error)
	DangerousGetReconcilerConfigValues(ctx context.Context, reconcilerName sqlc.ReconcilerName) (*ReconcilerConfigValues, error)
	GetAuditLogsForReconciler(ctx context.Context, reconcilerName sqlc.ReconcilerName, offset, limit int) ([]*AuditLog, int, error)
	SetLastSuccessfulSyncForTeam(ctx context.Context, teamSlug slug.Slug) error
	RevokeGlobalUserRole(ctx context.Context, userID uuid.UUID, roleName sqlc.RoleName) error
	GetUsersWithGloballyAssignedRole(ctx context.Context, roleName sqlc.RoleName) ([]*User, error)
	IsFirstRun(ctx context.Context) (bool, error)
	FirstRunComplete(ctx context.Context) error
	GetSlackAlertsChannels(ctx context.Context, teamSlug slug.Slug) (map[string]string, error)
	SetSlackAlertsChannel(ctx context.Context, teamSlug slug.Slug, environment, channelName string) error
	RemoveSlackAlertsChannel(ctx context.Context, teamSlug slug.Slug, environment string) error
	CreateTeamDeleteKey(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*TeamDeleteKey, error)
	GetTeamDeleteKey(ctx context.Context, key uuid.UUID) (*TeamDeleteKey, error)
	ConfirmTeamDeleteKey(ctx context.Context, key uuid.UUID) error
	DeleteTeam(ctx context.Context, teamSlug slug.Slug) error
	GetAuditLogsForCorrelationID(ctx context.Context, correlationID uuid.UUID, offset, limit int) ([]*AuditLog, int, error)
	AddReconcilerOptOut(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, reconcilerName sqlc.ReconcilerName) error
	RemoveReconcilerOptOut(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, reconcilerName sqlc.ReconcilerName) error
	GetTeamMembersForReconciler(ctx context.Context, teamSlug slug.Slug, reconcilerName sqlc.ReconcilerName) ([]*User, error)
	GetTeamMemberOptOuts(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) ([]*sqlc.GetTeamMemberOptOutsRow, error)
	GetTeamsWithPermissionInGitHubRepo(ctx context.Context, repoName, permission string, offset, limit int) ([]*Team, int, error)
	GetRepositoryAuthorizations(ctx context.Context, teamSlug slug.Slug, repo string) ([]sqlc.RepositoryAuthorizationEnum, error)
}

func (u User) GetID() uuid.UUID {
	return u.ID
}

func (u User) Identity() string {
	return u.Email
}

func (u User) IsServiceAccount() bool {
	return false
}

func (s ServiceAccount) GetID() uuid.UUID {
	return s.ID
}

func (s ServiceAccount) Identity() string {
	return s.Name
}

func (s ServiceAccount) IsServiceAccount() bool {
	return true
}

func (k TeamDeleteKey) Expires() time.Time {
	return k.CreatedAt.Add(teamDeleteKeyLifetime)
}

func (k TeamDeleteKey) HasExpired() bool {
	return time.Now().After(k.Expires())
}
