package graphv1

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v1/deployment"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
)

func (r *deploymentResolver) Team(ctx context.Context, obj *deployment.Deployment) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *deploymentResolver) Environment(ctx context.Context, obj *deployment.Deployment) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

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

	return deployment.KeyForTeam(ctx, obj.Slug)
}

func (r *teamResolver) Deployments(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*deployment.Deployment], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return deployment.ListForTeam(ctx, obj.Slug, page)
}

func (r *Resolver) Deployment() gengqlv1.DeploymentResolver { return &deploymentResolver{r} }

type deploymentResolver struct{ *Resolver }
