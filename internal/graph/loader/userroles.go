package loader

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"k8s.io/utils/ptr"
)

type userRolesReader struct {
	db database.RoleRepo
}

func (t userRolesReader) getUserRoles(ctx context.Context, userIDs []uuid.UUID) ([][]*model.Role, []error) {
	roles, err := t.db.GetUserRolesForUsers(ctx, userIDs)
	if err != nil {
		return nil, dupErrs(len(userIDs), err)
	}

	errs := make([]error, len(userIDs))
	ret := make([][]*model.Role, len(userIDs))
	for i, userID := range userIDs {
		if roles, ok := roles[userID]; ok {
			ret[i] = toGraphUserRoleList(roles)
		} else {
			errs[i] = pgx.ErrNoRows
		}
	}

	return ret, nil
}

func GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*model.Role, error) {
	return For(ctx).UserRolesLoader.Load(ctx, userID)
}

func ToGraphUserRoles(m *authz.Role) *model.Role {
	ret := &model.Role{
		Name:           string(m.RoleName),
		IsGlobal:       m.IsGlobal(),
		TargetTeamSlug: m.TargetTeamSlug,
	}

	if m.TargetServiceAccountID != nil {
		ret.TargetServiceAccountID = ptr.To(scalar.ServiceAccountIdent(*m.TargetServiceAccountID))
	}

	return ret
}

func toGraphUserRoleList(m []*authz.Role) []*model.Role {
	ret := make([]*model.Role, len(m))
	for i, v := range m {
		ret[i] = ToGraphUserRoles(v)
	}
	return ret
}
