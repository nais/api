package authz

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/auth/authz/authzsql"
	"github.com/nais/api/internal/slug"
)

func AssignTeamRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, teamSlug slug.Slug, roleName string) error {
	return db(ctx).AssignTeamRoleToServiceAccount(ctx, authzsql.AssignTeamRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
		TargetTeamSlug:   teamSlug,
	})
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

func ForServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) ([]*Role, error) {
	sar, err := fromContext(ctx).serviceAccountRoles.Load(ctx, serviceAccountID)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return []*Role{}, nil
	} else if err != nil {
		return nil, err
	}
	return sar.Roles, nil
}

func AssignGlobalRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName string) error {
	return db(ctx).AssignGlobalRoleToServiceAccount(ctx, authzsql.AssignGlobalRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
	})
}

func AssignDefaultPermissionsToUser(ctx context.Context, userID uuid.UUID) error {
	defaultUserRoles := []string{
		"Team creator",
		"Team viewer",
		"User viewer",
	}
	for _, roleName := range defaultUserRoles {
		if err := assignGlobalRoleToUser(ctx, userID, roleName); err != nil {
			return err
		}
	}
	return nil
}

func AssignGlobalAdmin(ctx context.Context, userID uuid.UUID) error {
	return db(ctx).AssignGlobalRoleToUser(ctx, authzsql.AssignGlobalRoleToUserParams{
		UserID:   userID,
		RoleName: "Admin",
	})
}

func MakeUserTeamMember(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error {
	panic("not implemented")
}

func MakeUserTeamOwner(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error {
	panic("not implemented")
}

func CanCreateServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	panic("not implemented")
}

func CanUpdateServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	panic("not implemented")
}

func CanDeleteServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	panic("not implemented")
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

func CanCreateTeam(ctx context.Context) error {
	return requireGlobalAuthorization(ctx, "teams:create")
}

func CanUpdateTeam(ctx context.Context, teamSlug slug.Slug) error {
	// TODO: authorization does not yet exist, create, or check for team owner role for the team?
	return requireTeamAuthorization(ctx, teamSlug, "teams:update")
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

func RevokeGlobalAdmin(ctx context.Context, userID uuid.UUID) error {
	return db(ctx).RevokeGlobalAdmin(ctx, userID)
}

func RequireGlobalAdminCtx(ctx context.Context) error {
	return RequireGlobalAdmin(ctx, ActorFromContext(ctx).User.GetID())
}

func RequireGlobalAdmin(ctx context.Context, userID uuid.UUID) error {
	if isAdmin, err := db(ctx).IsAdmin(ctx, userID); err != nil {
		return err
	} else if !isAdmin {
		return ErrUnauthorized
	}

	return nil
}

func assignGlobalRoleToUser(ctx context.Context, userID uuid.UUID, roleName string) error {
	return db(ctx).AssignGlobalRoleToUser(ctx, authzsql.AssignGlobalRoleToUserParams{
		UserID:   userID,
		RoleName: roleName,
	})
}

func requireTeamAuthorization(ctx context.Context, teamSlug slug.Slug, authorizationName string) error {
	hasAuthorization, err := db(ctx).HasTeamAuthorization(ctx, authzsql.HasTeamAuthorizationParams{
		UserID:            ActorFromContext(ctx).User.GetID(),
		AuthorizationName: authorizationName,
		TeamSlug:          teamSlug,
	})
	if err != nil {
		return err
	}

	if hasAuthorization {
		return nil
	}

	return newMissingAuthorizationError(authorizationName)
}

func requireGlobalAuthorization(ctx context.Context, authorizationName string) error {
	authorized, err := db(ctx).HasGlobalAuthorization(ctx, authzsql.HasGlobalAuthorizationParams{
		UserID:            ActorFromContext(ctx).User.GetID(),
		AuthorizationName: authorizationName,
	})
	if err != nil {
		return err
	}

	if authorized {
		return nil
	}

	return newMissingAuthorizationError(authorizationName)
}
