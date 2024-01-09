package database

import (
	"context"

	"github.com/nais/api/internal/auth/roles"

	"github.com/nais/api/internal/slug"

	"github.com/google/uuid"
	sqlc "github.com/nais/api/internal/database/gensql"
)

func (d *database) AssignGlobalRoleToUser(ctx context.Context, userID uuid.UUID, roleName sqlc.RoleName) error {
	return d.querier.AssignGlobalRoleToUser(ctx, sqlc.AssignGlobalRoleToUserParams{
		UserID:   userID,
		RoleName: roleName,
	})
}

func (d *database) AssignGlobalRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName sqlc.RoleName) error {
	return d.querier.AssignGlobalRoleToServiceAccount(ctx, sqlc.AssignGlobalRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
	})
}

func (d *database) AssignTeamRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName sqlc.RoleName, teamSlug slug.Slug) error {
	return d.querier.AssignTeamRoleToServiceAccount(ctx, sqlc.AssignTeamRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
		TargetTeamSlug:   &teamSlug,
	})
}

// IsGlobal Check if the role is globally assigned or not
func (r Role) IsGlobal() bool {
	return r.TargetServiceAccountID == nil && r.TargetTeamSlug == nil
}

// TargetsTeam Check if the role targets a specific team
func (r Role) TargetsTeam(targetsTeamSlug slug.Slug) bool {
	return r.TargetTeamSlug != nil && *r.TargetTeamSlug == targetsTeamSlug
}

// TargetsServiceAccount Check if the role targets a specific service account
func (r Role) TargetsServiceAccount(targetServiceAccountID uuid.UUID) bool {
	return r.TargetServiceAccountID != nil && *r.TargetServiceAccountID == targetServiceAccountID
}

func (d *database) UserIsTeamOwner(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) (bool, error) {
	roles, err := d.querier.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if role.RoleName == sqlc.RoleNameTeamowner && role.TargetTeamSlug != nil && *role.TargetTeamSlug == teamSlug {
			return true, nil
		}
	}

	return false, nil
}

func (d *database) SetTeamMemberRole(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, role sqlc.RoleName) error {
	return d.querier.AssignTeamRoleToUser(ctx, sqlc.AssignTeamRoleToUserParams{
		UserID:         userID,
		TargetTeamSlug: &teamSlug,
		RoleName:       role,
	})
}

func (d *database) RevokeGlobalUserRole(ctx context.Context, userID uuid.UUID, roleName sqlc.RoleName) error {
	return d.querier.RevokeGlobalUserRole(ctx, sqlc.RevokeGlobalUserRoleParams{
		RoleName: roleName,
		UserID:   userID,
	})
}

func (d *database) GetUsersWithGloballyAssignedRole(ctx context.Context, roleName sqlc.RoleName) ([]*User, error) {
	users, err := d.querier.GetUsersWithGloballyAssignedRole(ctx, roleName)
	if err != nil {
		return nil, err
	}

	return wrapUsers(users), nil
}

func (d *database) GetAllUserRoles(ctx context.Context) ([]*UserRole, error) {
	userRoles, err := d.querier.GetAllUserRoles(ctx)
	if err != nil {
		return nil, err
	}

	roles := make([]*UserRole, 0, len(userRoles))
	for _, userRole := range userRoles {
		roles = append(roles, &UserRole{userRole})
	}

	return roles, nil
}

func (d *database) roleFromRoleBinding(_ context.Context, roleName sqlc.RoleName, targetServiceAccountID *uuid.UUID, targetTeamSlug *slug.Slug) (*Role, error) {
	authorizations, err := roles.Authorizations(roleName)
	if err != nil {
		return nil, err
	}

	return &Role{
		Authorizations:         authorizations,
		RoleName:               roleName,
		TargetServiceAccountID: targetServiceAccountID,
		TargetTeamSlug:         targetTeamSlug,
	}, nil
}
