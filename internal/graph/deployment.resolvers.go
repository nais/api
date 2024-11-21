package graph

import (
	"context"
	"log"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/deployment"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) DeploymentInfo(ctx context.Context, obj *application.Application) (*deployment.DeploymentInfo, error) {
	return deployment.InfoForWorkload(ctx, obj)
}

func (r *deploymentResolver) Team(ctx context.Context, obj *deployment.Deployment) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *deploymentResolver) Environment(ctx context.Context, obj *deployment.Deployment) (*team.TeamEnvironment, error) {
	env, err := team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
	if err != nil {
		log.Println(err, obj.TeamSlug, obj.EnvironmentName)
		return nil, err
	}

	return env, nil
}

func (r *deploymentInfoResolver) History(ctx context.Context, obj *deployment.DeploymentInfo, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*deployment.Deployment], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return deployment.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.WorkloadName, obj.WorkloadType, page)
}

func (r *jobResolver) DeploymentInfo(ctx context.Context, obj *job.Job) (*deployment.DeploymentInfo, error) {
	return deployment.InfoForWorkload(ctx, obj)
}

func (r *mutationResolver) ChangeDeploymentKey(ctx context.Context, input deployment.ChangeDeploymentKeyInput) (*deployment.ChangeDeploymentKeyPayload, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationDeployKeyUpdate, input.TeamSlug); err != nil {
		return nil, err
	}

	dk, err := deployment.ChangeDeploymentKey(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	return &deployment.ChangeDeploymentKeyPayload{DeploymentKey: dk}, nil
}

func (r *teamResolver) DeploymentKey(ctx context.Context, obj *team.Team) (*deployment.DeploymentKey, error) {
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationDeployKeyRead, obj.Slug); err != nil {
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

func (r *Resolver) Deployment() gengql.DeploymentResolver { return &deploymentResolver{r} }

func (r *Resolver) DeploymentInfo() gengql.DeploymentInfoResolver { return &deploymentInfoResolver{r} }

type (
	deploymentResolver     struct{ *Resolver }
	deploymentInfoResolver struct{ *Resolver }
)
