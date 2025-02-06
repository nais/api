package authz

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz/authzsql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type Role struct {
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	TargetTeamSlug *slug.Slug `json:"target_team_slug"`
}

type (
	RoleConnection = pagination.Connection[*Role]
	RoleEdge       = pagination.Edge[*Role]
)

// IsGlobal checks if the role is globally assigned.
func (r *Role) IsGlobal() bool {
	return r.TargetTeamSlug == nil
}

// TargetsTeam checks if the role targets a specific team.
func (r *Role) TargetsTeam(targetsTeamSlug slug.Slug) bool {
	return r.TargetTeamSlug != nil && *r.TargetTeamSlug == targetsTeamSlug
}

type UserRoles struct {
	UserID uuid.UUID
	Roles  []*Role
}

func toUserRoles(row *authzsql.GetRolesForUsersRow) (*UserRoles, error) {
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

func toServiceAccountRoles(row *authzsql.GetRolesForServiceAccountsRow) (*ServiceAccountRoles, error) {
	var roles []*Role
	if err := json.Unmarshal(row.Roles, &roles); err != nil {
		return nil, err
	}

	return &ServiceAccountRoles{
		ServiceAccountID: row.ServiceAccountID,
		Roles:            roles,
	}, nil
}
