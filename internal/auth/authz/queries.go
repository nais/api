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
	panic("not implemented")
}

func MakeUserTeamMember(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error {
	panic("not implemented")
}

func MakeUserTeamOwner(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error {
	panic("not implemented")
}

func CanCreateServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanUpdateServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanDeleteServiceAccounts(ctx context.Context, teamSlug *slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanReadDeployKey(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanUpdateDeployKey(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanDeleteApplications(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanUpdateApplications(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanDeleteJobs(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanUpdateJobs(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanCreateRepositories(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanDeleteRepositories(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanCreateSecrets(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanReadSecrets(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanUpdateSecrets(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanDeleteSecrets(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanCreateTeam(ctx context.Context) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanUpdateTeam(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanUpdateTeamMetadata(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanDeleteTeam(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanCreateUnleash(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func CanUpdateUnleash(ctx context.Context, teamSlug slug.Slug) error {
	// actor := authz.ActorFromContext(ctx)
	panic("not implemented")
}

func RevokeGlobalAdmin(ctx context.Context, userID uuid.UUID) error {
	panic("not implemented")
}

func IsGlobalAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	// don't return error if user is not admin
	panic("not implemented")
}

func assignGlobalRoleToUser(ctx context.Context, userID uuid.UUID, roleName string) error {
	return db(ctx).AssignGlobalRoleToUser(ctx, authzsql.AssignGlobalRoleToUserParams{
		UserID:   userID,
		RoleName: roleName,
	})
}

func assignTeamRoleToUser(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, roleName string) error {
	return db(ctx).AssignTeamRoleToUser(ctx, authzsql.AssignTeamRoleToUserParams{
		UserID:         userID,
		RoleName:       roleName,
		TargetTeamSlug: teamSlug,
	})
}
