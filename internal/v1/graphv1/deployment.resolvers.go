package graphv1

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v1/deployment"
	"github.com/nais/api/internal/v1/team"
)

func (r *mutationResolver) ChangeDeploymentKey(ctx context.Context, input deployment.ChangeDeploymentKeyInput) (*deployment.ChangeDeploymentKeyPayload, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	dk, err := deployment.ChangeDeploymentKey(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	return &deployment.ChangeDeploymentKeyPayload{DeploymentKey: dk}, nil
}

func (r *teamResolver) DeploymentKey(ctx context.Context, obj *team.Team) (*deployment.DeploymentKey, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, obj.Slug); err != nil {
		return nil, err
	}

	return deployment.ForTeam(ctx, obj.Slug)
}
