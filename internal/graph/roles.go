package graph

import (
	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
)

func toGraphRoles(roles []*authz.Role) []*model.Role {
	ret := make([]*model.Role, len(roles))
	for i, role := range roles {
		var saID uuid.UUID
		if role.TargetServiceAccountID != nil {
			saID = *role.TargetServiceAccountID
		}
		ret[i] = &model.Role{
			Name:     string(role.RoleName),
			IsGlobal: role.IsGlobal(),
			GQLVars: model.RoleGQLVars{
				TargetServiceAccountID: saID,
				TargetTeamSlug:         role.TargetTeamSlug,
			},
		}
	}

	return ret
}
