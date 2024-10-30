package role

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/role/rolesql"
)

type UserRoles struct {
	UserID uuid.UUID
	Roles  []*Role
}

func toUserRoles(row *rolesql.GetRolesForUsersRow) (*UserRoles, error) {
	var roles []*Role
	if err := json.Unmarshal(row.Roles, &roles); err != nil {
		return nil, err
	}

	return &UserRoles{
		UserID: row.UserID,
		Roles:  roles,
	}, nil
}

type Role struct {
	Name           rolesql.RoleName `json:"role_name"`
	TargetTeamSlug slug.Slug        `json:"target_team_slug"`
}
