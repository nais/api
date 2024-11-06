package role

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/role/rolesql"
	"github.com/nais/api/internal/slug"
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

func AssignGlobalRoleToUser(ctx context.Context, userID uuid.UUID, roleName rolesql.RoleName) error {
	return db(ctx).AssignGlobalRoleToUser(ctx, rolesql.AssignGlobalRoleToUserParams{
		UserID:   userID,
		RoleName: roleName,
	})
}

func AssignGlobalRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName rolesql.RoleName) error {
	return db(ctx).AssignGlobalRoleToServiceAccount(ctx, rolesql.AssignGlobalRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
	})
}
