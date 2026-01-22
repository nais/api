package authz

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/auth/authz/authzsql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
)

func ListRoles(ctx context.Context, page *pagination.Pagination) (*RoleConnection, error) {
	q := db(ctx)

	ret, err := q.ListRoles(ctx, authzsql.ListRolesParams{
		Offset: page.Offset(),
		Limit:  page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountRoles(ctx)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, total, toGraphRole), nil
}

func ListRolesForServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, page *pagination.Pagination) (*RoleConnection, error) {
	q := db(ctx)

	ret, err := q.ListRolesForServiceAccount(ctx, authzsql.ListRolesForServiceAccountParams{
		Offset:           page.Offset(),
		Limit:            page.Limit(),
		ServiceAccountID: serviceAccountID,
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountRolesForServiceAccount(ctx, serviceAccountID)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, total, toGraphRole), nil
}

func getRoleByIdent(ctx context.Context, id ident.Ident) (*Role, error) {
	name, err := parseRoleIdent(id)
	if err != nil {
		return nil, err
	}

	row, err := db(ctx).GetRoleByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return toGraphRole(row), nil
}

func ForUser(ctx context.Context, userID uuid.UUID) ([]*Role, error) {
	ur, err := fromContext(ctx).userRoles.Load(ctx, userID)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return []*Role{}, nil
	} else if err != nil {
		return nil, err
	}
	return ur.Roles, nil
}

func ForGitHubRepo(ctx context.Context, teamSlug slug.Slug) ([]*Role, error) {
	role, err := GetRole(ctx, "GitHub repository")
	if err != nil {
		return nil, err
	}

	role.TargetTeamSlug = ptr.To(teamSlug)
	return []*Role{role}, nil
}

func ForServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) ([]*Role, error) {
	sar, err := fromContext(ctx).serviceAccountRoles.Load(ctx, serviceAccountID)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return []*Role{}, nil
	} else if err != nil {
		return nil, err
	}
	return sar.Roles, nil
}

func AssignRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName string) error {
	return db(ctx).AssignRoleToServiceAccount(ctx, authzsql.AssignRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
	})
}

func RevokeRoleFromServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName string) error {
	return db(ctx).RevokeRoleFromServiceAccount(ctx, authzsql.RevokeRoleFromServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
	})
}

func MakeUserTeamMember(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error {
	return db(ctx).AssignTeamRoleToUser(ctx, authzsql.AssignTeamRoleToUserParams{
		UserID:         userID,
		RoleName:       "Team member",
		TargetTeamSlug: teamSlug,
	})
}

func MakeUserTeamOwner(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error {
	return db(ctx).AssignTeamRoleToUser(ctx, authzsql.AssignTeamRoleToUserParams{
		UserID:         userID,
		RoleName:       "Team owner",
		TargetTeamSlug: teamSlug,
	})
}

func GetRole(ctx context.Context, name string) (*Role, error) {
	row, err := db(ctx).GetRoleByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return toGraphRole(row), nil
}

func ServiceAccountHasRole(ctx context.Context, serviceAccountID uuid.UUID, roleName string) (bool, error) {
	return db(ctx).ServiceAccountHasRole(ctx, authzsql.ServiceAccountHasRoleParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
	})
}

func CanAssignRole(ctx context.Context, roleName string, targetTeamSlug *slug.Slug) (bool, error) {
	actor := ActorFromContext(ctx)
	if actor.User.IsServiceAccount() {
		return serviceAccountCanAssignRole(ctx, actor.User.GetID(), roleName, targetTeamSlug)
	}

	return userCanAssignRole(ctx, actor.User.GetID(), roleName, targetTeamSlug)
}

func userCanAssignRole(ctx context.Context, userID uuid.UUID, roleName string, targetTeamSlug *slug.Slug) (bool, error) {
	return db(ctx).UserCanAssignRole(ctx, authzsql.UserCanAssignRoleParams{
		UserID:         userID,
		RoleName:       roleName,
		TargetTeamSlug: targetTeamSlug,
	})
}

func serviceAccountCanAssignRole(ctx context.Context, serviceAccountID uuid.UUID, roleName string, targetTeamSlug *slug.Slug) (bool, error) {
	return db(ctx).ServiceAccountCanAssignRole(ctx, authzsql.ServiceAccountCanAssignRoleParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
		TeamSlug:         targetTeamSlug,
	})
}

func CanCreateServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	return requireAuthorization(ctx, "service_accounts:create", teamSlug)
}

func CanUpdateServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	return requireAuthorization(ctx, "service_accounts:update", teamSlug)
}

func CanDeleteServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	return requireAuthorization(ctx, "service_accounts:delete", teamSlug)
}

func CanReadDeployKey(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "deploy_key:read")
}

func CanUpdateDeployKey(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "deploy_key:update")
}

func CanDeleteApplications(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "applications:delete")
}

func CanUpdateApplications(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "applications:update")
}

func CanDeleteJobs(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "jobs:delete")
}

func CanUpdateJobs(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "jobs:update")
}

func CanCreateRepositories(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "repositories:create")
}

func CanDeleteRepositories(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "repositories:delete")
}

func CanCreateSecrets(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "teams:secrets:create")
}

func CanReadSecrets(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "teams:secrets:read")
}

func CanUpdateSecrets(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "teams:secrets:update")
}

func CanDeleteSecrets(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "teams:secrets:delete")
}

// CanCreateElevation checks if the user can create an elevation for the team.
// This enforces strict team membership WITHOUT admin bypass for security reasons.
func CanCreateElevation(ctx context.Context, teamSlug slug.Slug) error {
	return requireStrictTeamAuthorization(ctx, teamSlug, "teams:elevations:create")
}

// CanReadSecretValues checks if the user can read secret values for the team.
// This enforces strict team membership WITHOUT admin bypass for security reasons.
func CanReadSecretValues(ctx context.Context, teamSlug slug.Slug) error {
	return requireStrictTeamAuthorization(ctx, teamSlug, "teams:secrets:read-values")
}

func CanCreateTeam(ctx context.Context) error {
	return requireGlobalAuthorization(ctx, "teams:create")
}

func CanManageTeamMembers(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "teams:members:admin")
}

func CanUpdateTeamMetadata(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "teams:metadata:update")
}

func CanDeleteTeam(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "teams:delete")
}

func CanCreateUnleash(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "unleash:create")
}

func CanUpdateUnleash(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "unleash:update")
}

func CanDeleteUnleash(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "unleash:delete")
}

func CanUpdateVulnerability(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "vulnerability:update")
}

func CanStartServiceMaintenance(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "service_maintenance:update:start")
}

func CanCreateValkey(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "valkeys:create")
}

func CanUpdateValkey(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "valkeys:update")
}

func CanDeleteValkey(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "valkeys:delete")
}

func CanCreateOpenSearch(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "opensearches:create")
}

func CanUpdateOpenSearch(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "opensearches:update")
}

func CanDeleteOpenSearch(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "opensearches:delete")
}

func CanGrantPostgresAccess(ctx context.Context, teamSlug slug.Slug) error {
	return requireTeamAuthorization(ctx, teamSlug, "postgres:access:grant")
}

func RequireGlobalAdmin(ctx context.Context) error {
	if ActorFromContext(ctx).User.IsAdmin() {
		return nil
	}

	return ErrUnauthorized
}

// requireTeamAuthorization enforces team authorization WITH admin bypass.
// Admins can perform actions even if they're not team members.
// This also allows global roles (target_team_slug IS NULL) to apply.
// Use this for normal team operations like updating apps, managing resources, etc.
func requireTeamAuthorization(ctx context.Context, teamSlug slug.Slug, authorizationName string) error {
	actor := ActorFromContext(ctx)
	user := actor.User
	var (
		hasAuthorization bool
		err              error
	)

	type githubActions interface {
		IsGitHubActions()
	}
	if user.IsServiceAccount() {
		if _, ok := user.(githubActions); ok {
			// This is a cheat to support OIDC from GitHubActions. See middleware.GitHubOIDC for how the roles are assigned
			return isAuthorizedThroughRoles(ctx, authorizationName, teamSlug, actor.Roles)
		}

		hasAuthorization, err = db(ctx).ServiceAccountHasTeamAuthorization(ctx, authzsql.ServiceAccountHasTeamAuthorizationParams{
			ServiceAccountID:  user.GetID(),
			AuthorizationName: authorizationName,
			TeamSlug:          teamSlug,
		})
	} else {
		hasAuthorization, err = db(ctx).HasTeamAuthorization(ctx, authzsql.HasTeamAuthorizationParams{
			UserID:            user.GetID(),
			AuthorizationName: authorizationName,
			TeamSlug:          teamSlug,
		})
	}
	if err != nil {
		return err
	}

	if hasAuthorization {
		return nil
	}

	return newMissingAuthorizationError(authorizationName)
}

// requireStrictTeamAuthorization enforces team authorization WITHOUT admin bypass.
// Even admins MUST be team members to perform these security-sensitive operations.
// This does NOT allow global roles - the user must have the role on the specific team.
// Use this for operations that:
// - Create audit trail (elevations)
// - Access sensitive data (secret values)
// - Should never bypass team membership for security reasons
func requireStrictTeamAuthorization(ctx context.Context, teamSlug slug.Slug, authorizationName string) error {
	actor := ActorFromContext(ctx)
	user := actor.User
	var (
		hasAuthorization bool
		err              error
	)

	type githubActions interface {
		IsGitHubActions()
	}
	if user.IsServiceAccount() {
		if _, ok := user.(githubActions); ok {
			// This is a cheat to support OIDC from GitHubActions. See middleware.GitHubOIDC for how the roles are assigned
			return isAuthorizedThroughRoles(ctx, authorizationName, teamSlug, actor.Roles)
		}

		hasAuthorization, err = db(ctx).ServiceAccountHasTeamMembership(ctx, authzsql.ServiceAccountHasTeamMembershipParams{
			ServiceAccountID:  user.GetID(),
			AuthorizationName: authorizationName,
			TeamSlug:          teamSlug,
		})
	} else {
		hasAuthorization, err = db(ctx).HasTeamMembership(ctx, authzsql.HasTeamMembershipParams{
			UserID:            user.GetID(),
			AuthorizationName: authorizationName,
			TeamSlug:          teamSlug,
		})
	}
	if err != nil {
		return err
	}

	if hasAuthorization {
		return nil
	}

	return newMissingAuthorizationError(authorizationName)
}

func requireGlobalAuthorization(ctx context.Context, authorizationName string) error {
	user := ActorFromContext(ctx).User
	var (
		authorized bool
		err        error
	)
	if user.IsServiceAccount() {
		authorized, err = db(ctx).ServiceAccountHasGlobalAuthorization(ctx, authzsql.ServiceAccountHasGlobalAuthorizationParams{
			ServiceAccountID:  user.GetID(),
			AuthorizationName: authorizationName,
		})
	} else {
		authorized, err = db(ctx).HasGlobalAuthorization(ctx, authzsql.HasGlobalAuthorizationParams{
			UserID:            user.GetID(),
			AuthorizationName: authorizationName,
		})
	}
	if err != nil {
		return err
	}

	if authorized {
		return nil
	}

	return newMissingAuthorizationError(authorizationName)
}

func requireAuthorization(ctx context.Context, authorizationName string, teamSlug *slug.Slug) error {
	if teamSlug == nil {
		return requireGlobalAuthorization(ctx, authorizationName)
	}

	return requireTeamAuthorization(ctx, *teamSlug, authorizationName)
}

func isAuthorizedThroughRoles(ctx context.Context, authorizationName string, teamSlug slug.Slug, roles []*Role) error {
	for _, r := range roles {
		matchesTeamSlug := r.TargetTeamSlug != nil && *r.TargetTeamSlug == teamSlug
		if !matchesTeamSlug {
			continue
		}

		ok, err := db(ctx).GitHubAuthorizationRoleCheck(ctx, authzsql.GitHubAuthorizationRoleCheckParams{
			RoleName:          r.Name,
			AuthorizationName: authorizationName,
		})
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}

	return newMissingAuthorizationError(authorizationName)
}
