package graph

import (
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"k8s.io/utils/ptr"
)

func toGraphRoles(roles []*authz.Role) []*model.Role {
	ret := make([]*model.Role, 0, len(roles))
	for _, role := range roles {
		a := &model.Role{
			Name:           string(role.RoleName),
			TargetTeamSlug: role.TargetTeamSlug,
			IsGlobal:       role.IsGlobal(),
		}

		if role.TargetServiceAccountID != nil {
			a.TargetServiceAccountID = ptr.To(scalar.ServiceAccountIdent(*role.TargetServiceAccountID))
		}

		ret = append(ret, a)
	}

	return ret
}
