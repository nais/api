package role

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/role/rolesql"
)

func AssignTeamRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, teamSlug slug.Slug, roleName rolesql.RoleName) error {
	return db(ctx).AssignTeamRoleToServiceAccount(ctx, rolesql.AssignTeamRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
		TargetTeamSlug:   teamSlug,
	})
}

func AssignTeamRoleToUser(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, roleName rolesql.RoleName) error {
	return db(ctx).AssignTeamRoleToUser(ctx, rolesql.AssignTeamRoleToUserParams{
		UserID:         userID,
		RoleName:       roleName,
		TargetTeamSlug: teamSlug,
	})
}

func ForUser(ctx context.Context, userID uuid.UUID) ([]*Role, error) {
	ur, err := fromContext(ctx).userRoles.Load(ctx, userID)
	if err != nil {
		return nil, err
	}
	return ur.Roles, nil
}
