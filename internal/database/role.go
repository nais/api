package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type RoleRepo interface {
	AssignGlobalRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName gensql.RoleName) error
	AssignGlobalRoleToUser(ctx context.Context, userID uuid.UUID, roleName gensql.RoleName) error
	AssignTeamRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName gensql.RoleName, teamSlug slug.Slug) error
	GetAllUserRoles(ctx context.Context) ([]*UserRole, error)
	GetUserRolesForUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID][]*authz.Role, error)
	GetUsersWithGloballyAssignedRole(ctx context.Context, roleName gensql.RoleName) ([]*User, error)
	RevokeGlobalUserRole(ctx context.Context, userID uuid.UUID, roleName gensql.RoleName) error
	SetTeamMemberRole(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, roleName gensql.RoleName) error
	UserIsTeamOwner(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) (bool, error)
}

var _ RoleRepo = (*database)(nil)

func (d *database) AssignGlobalRoleToUser(ctx context.Context, userID uuid.UUID, roleName gensql.RoleName) error {
	return d.querier.AssignGlobalRoleToUser(ctx, gensql.AssignGlobalRoleToUserParams{
		UserID:   userID,
		RoleName: roleName,
	})
}

func (d *database) AssignGlobalRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName gensql.RoleName) error {
	return d.querier.AssignGlobalRoleToServiceAccount(ctx, gensql.AssignGlobalRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
	})
}

func (d *database) AssignTeamRoleToServiceAccount(ctx context.Context, serviceAccountID uuid.UUID, roleName gensql.RoleName, teamSlug slug.Slug) error {
	return d.querier.AssignTeamRoleToServiceAccount(ctx, gensql.AssignTeamRoleToServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		RoleName:         roleName,
		TargetTeamSlug:   teamSlug,
	})
}

func (d *database) UserIsTeamOwner(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) (bool, error) {
	roles, err := d.querier.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if role.RoleName == gensql.RoleNameTeamowner && role.TargetTeamSlug != nil && *role.TargetTeamSlug == teamSlug {
			return true, nil
		}
	}

	return false, nil
}

func (d *database) SetTeamMemberRole(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, roleName gensql.RoleName) error {
	return d.querier.AssignTeamRoleToUser(ctx, gensql.AssignTeamRoleToUserParams{
		UserID:         userID,
		RoleName:       roleName,
		TargetTeamSlug: teamSlug,
	})
}

func (d *database) RevokeGlobalUserRole(ctx context.Context, userID uuid.UUID, roleName gensql.RoleName) error {
	return d.querier.RevokeGlobalUserRole(ctx, gensql.RevokeGlobalUserRoleParams{
		UserID:   userID,
		RoleName: roleName,
	})
}

func (d *database) GetUsersWithGloballyAssignedRole(ctx context.Context, roleName gensql.RoleName) ([]*User, error) {
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

	ret := make([]*UserRole, len(userRoles))
	for i, userRole := range userRoles {
		ret[i] = &UserRole{userRole}
	}

	return ret, nil
}

func (d *database) GetUserRolesForUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID][]*authz.Role, error) {
	usersWithRoles, err := d.querier.GetUserRolesForUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	ret := make(map[uuid.UUID][]*authz.Role)
	for _, user := range usersWithRoles {
		role, err := d.roleFromRoleBinding(ctx, user.RoleName, user.TargetServiceAccountID, user.TargetTeamSlug)
		if err != nil {
			continue
		}
		ret[user.UserID] = append(ret[user.UserID], role)
	}

	return ret, nil
}

func (d *database) roleFromRoleBinding(_ context.Context, roleName gensql.RoleName, targetServiceAccountID *uuid.UUID, targetTeamSlug *slug.Slug) (*authz.Role, error) {
	authorizations, err := roles.Authorizations(roleName)
	if err != nil {
		return nil, err
	}

	return &authz.Role{
		Authorizations:         authorizations,
		RoleName:               roleName,
		TargetServiceAccountID: targetServiceAccountID,
		TargetTeamSlug:         targetTeamSlug,
	}, nil
}
