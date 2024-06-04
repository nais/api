package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
)

func (r *roleResolver) TargetServiceAccount(ctx context.Context, obj *model.Role) (*model.ServiceAccount, error) {
	if obj.GQLVars.TargetServiceAccountID == uuid.Nil {
		return nil, nil
	}
	return loader.GetServiceAccount(ctx, obj.GQLVars.TargetServiceAccountID)
}

func (r *roleResolver) TargetTeam(ctx context.Context, obj *model.Role) (*model.Team, error) {
	if obj.GQLVars.TargetTeamSlug == nil {
		return nil, nil
	}
	return loader.GetTeam(ctx, *obj.GQLVars.TargetTeamSlug)
}

func (r *serviceAccountResolver) Roles(ctx context.Context, obj *model.ServiceAccount) ([]*model.Role, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireRole(actor, gensql.RoleNameAdmin)
	if err != nil && actor.User.GetID() != obj.ID {
		return nil, err
	}

	roles, err := r.database.GetServiceAccountRoles(ctx, obj.ID)
	if err != nil {
		return nil, err
	}

	return toGraphRoles(roles), nil
}

func (r *Resolver) Role() gengql.RoleResolver { return &roleResolver{r} }

func (r *Resolver) ServiceAccount() gengql.ServiceAccountResolver { return &serviceAccountResolver{r} }

type (
	roleResolver           struct{ *Resolver }
	serviceAccountResolver struct{ *Resolver }
)
