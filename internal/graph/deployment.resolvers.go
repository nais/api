package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/deployment"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) Deployments(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*deployment.Deployment], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return deployment.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, workload.TypeApplication, page)
}

func (r *deploymentResolver) Resources(ctx context.Context, obj *deployment.Deployment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*deployment.DeploymentResource], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return deployment.ListResourcesForDeployment(ctx, obj.UUID, page)
}

func (r *deploymentResolver) Statuses(ctx context.Context, obj *deployment.Deployment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*deployment.DeploymentStatus], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return deployment.ListStatusesForDeployment(ctx, obj.UUID, page)
}

func (r *jobResolver) Deployments(ctx context.Context, obj *job.Job, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*deployment.Deployment], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return deployment.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name, workload.TypeJob, page)
}

func (r *mutationResolver) ChangeDeploymentKey(ctx context.Context, input deployment.ChangeDeploymentKeyInput) (*deployment.ChangeDeploymentKeyPayload, error) {
	if err := authz.CanUpdateDeployKey(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	dk, err := deployment.ChangeDeploymentKey(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	return &deployment.ChangeDeploymentKeyPayload{DeploymentKey: dk}, nil
}

func (r *queryResolver) Deployments(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *deployment.DeploymentFilter) (*pagination.Connection[*deployment.Deployment], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return deployment.List(ctx, page, filter)
}

func (r *teamResolver) DeploymentKey(ctx context.Context, obj *team.Team) (*deployment.DeploymentKey, error) {
	if err := authz.CanReadDeployKey(ctx, obj.Slug); err != nil {
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

type deploymentResolver struct{ *Resolver }
