package role

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/role/rolesql"
	"github.com/nais/api/internal/slug"
)

type Role struct {
	Name           string     `json:"role_name"`
	TargetTeamSlug *slug.Slug `json:"target_team_slug"`
}

// IsGlobal checks if the role is globally assigned.
func (r *Role) IsGlobal() bool {
	return r.TargetTeamSlug == nil
}

// TargetsTeam checks if the role targets a specific team.
func (r *Role) TargetsTeam(targetsTeamSlug slug.Slug) bool {
	return r.TargetTeamSlug != nil && *r.TargetTeamSlug == targetsTeamSlug
}

// Authorizations returns the authorizations for the role.
func (r *Role) Authorizations() ([]Authorization, error) {
	authorizations, exists := roles[r.Name]
	if !exists {
		return nil, fmt.Errorf("unknown role: %q", r.Name)
	}

	return authorizations, nil
}

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

type ServiceAccountRoles struct {
	ServiceAccountID uuid.UUID
	Roles            []*Role
}

func toServiceAccountRoles(row *rolesql.GetRolesForServiceAccountsRow) (*ServiceAccountRoles, error) {
	var roles []*Role
	if err := json.Unmarshal(row.Roles, &roles); err != nil {
		return nil, err
	}

	return &ServiceAccountRoles{
		ServiceAccountID: row.ServiceAccountID,
		Roles:            roles,
	}, nil
}
