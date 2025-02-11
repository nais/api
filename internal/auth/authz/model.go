package authz

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz/authzsql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type (
	RoleConnection = pagination.Connection[*Role]
	RoleEdge       = pagination.Edge[*Role]
)

type Role struct {
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	TargetTeamSlug *slug.Slug `json:"target_team_slug"`
	OnlyGlobal     bool       `json:"-"`
}

func (r *Role) ID() ident.Ident {
	return newRoleIdent(r.Name)
}
func (r *Role) IsNode() {}

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

func toGraphRole(row *authzsql.Role) *Role {
	return &Role{
		Name:        row.Name,
		Description: row.Description,
		OnlyGlobal:  row.IsOnlyGlobal,
	}
}
