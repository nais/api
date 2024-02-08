package graph

import (
	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
)

func toGraphRoles(roles []*authz.Role) []*model.Role {
	ret := make([]*model.Role, 0, len(roles))
	for _, role := range roles {
		var saID uuid.UUID
		if role.TargetServiceAccountID != nil {
			saID = *role.TargetServiceAccountID
		}
		a := &model.Role{
			Name:     string(role.RoleName),
			IsGlobal: role.IsGlobal(),
			GQLVars: model.RoleGQLVars{
				TargetServiceAccountID: saID,
				TargetTeamSlug:         role.TargetTeamSlug,
			},
		}

		ret = append(ret, a)
	}

	return ret
}
