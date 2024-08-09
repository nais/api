package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
)

const MaxDataPoints = 1000

func (r *appResolver) Persistence(ctx context.Context, obj *model.App) ([]model.Persistence, error) {
	return r.k8sClient.Persistence(ctx, obj.WorkloadBase)
}

func (r *appResolver) ImageDetails(ctx context.Context, obj *model.App) (*model.ImageDetails, error) {
	image, err := r.dependencyTrackClient.GetMetadataForImage(ctx, obj.Image)
	if err != nil {
		return nil, fmt.Errorf("getting metadata for image %q: %w", obj.Image, err)
	}

	return image, nil
}

func (r *appResolver) Instances(ctx context.Context, obj *model.App) ([]*model.Instance, error) {
	instances, err := r.k8sClient.Instances(ctx, obj.GQLVars.Team.String(), obj.Env.Name, obj.Name)
	if err != nil {
		return nil, fmt.Errorf("getting instances from Kubernetes: %w", err)
	}

	return instances, nil
}

func (r *appResolver) Manifest(ctx context.Context, obj *model.App) (string, error) {
	app, err := r.k8sClient.Manifest(ctx, obj.Name, obj.GQLVars.Team.String(), obj.Env.Name)
	if err != nil {
		return "", fmt.Errorf("getting app manifest from Kubernetes: %w", err)
	}
	return app, err
}

func (r *appResolver) Team(ctx context.Context, obj *model.App) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *appResolver) Secrets(ctx context.Context, obj *model.App) ([]*model.Secret, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, obj.GQLVars.Team)
	if err != nil {
		return nil, err
	}

	return r.k8sClient.SecretsForApp(ctx, obj)
}

func (r *appResolver) Utilization(ctx context.Context, obj *model.App) (*model.AppUtilization, error) {
	return &model.AppUtilization{
		GQLVars: model.AppUtilizationGQLVars{
			Env:     obj.Env.Name,
			Team:    obj.GQLVars.Team,
			AppName: obj.Name,
		},
	}, nil
}

func (r *appUtilizationResolver) Current(ctx context.Context, obj *model.AppUtilization, resourceType model.UtilizationResourceType) (float64, error) {
	return r.resourceUsageClientV2.CurrentResourceUtilizationForApp(ctx, obj.GQLVars.Env, obj.GQLVars.Team, obj.GQLVars.AppName, resourceType)
}

func (r *appUtilizationResolver) Request(ctx context.Context, obj *model.AppUtilization, resourceType model.UtilizationResourceType) (float64, error) {
	return r.resourceUsageClientV2.ResourceRequestForApp(ctx, obj.GQLVars.Env, obj.GQLVars.Team, obj.GQLVars.AppName, resourceType)
}

func (r *appUtilizationResolver) Range(ctx context.Context, obj *model.AppUtilization, start time.Time, end time.Time, step int, resourceType model.UtilizationResourceType) ([]*model.UtilizationDataPoint, error) {
	dpsRequested := ((int(end.Unix()) - int(start.Unix())) / step)
	if dpsRequested > MaxDataPoints {
		return nil, apierror.Errorf("maximum datapoints exceeded. Maximum allowed is %d, you requested %d", MaxDataPoints, dpsRequested)
	}

	// return r.resourceUsageClientV2.ResourceUtilizationForApp(ctx, obj.GQLVars.Env, obj.GQLVars.Team, obj.GQLVars.AppName, resourceType, start, end, step)
	return r.resourceUsageClientV2.ResourceUtilizationForApp(ctx, "dev", slug.Slug("etterlatte"), "etterlatte-klage", resourceType, start, end, step)
}

func (r *mutationResolver) DeleteApp(ctx context.Context, name string, team slug.Slug, env string) (*model.DeleteAppResult, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return nil, err
	}

	if err := r.k8sClient.DeleteApp(ctx, name, team.String(), env); err != nil {
		return &model.DeleteAppResult{
			Deleted: false,
			Error:   ptr.To(err.Error()),
		}, nil
	}

	return &model.DeleteAppResult{
		Deleted: true,
	}, nil
}

func (r *mutationResolver) RestartApp(ctx context.Context, name string, team slug.Slug, env string) (*model.RestartAppResult, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return nil, err
	}

	if err := r.k8sClient.RestartApp(ctx, name, team.String(), env); err != nil {
		return &model.RestartAppResult{
			Error: ptr.To(err.Error()),
		}, nil
	}
	return &model.RestartAppResult{}, nil
}

func (r *queryResolver) App(ctx context.Context, name string, team slug.Slug, env string) (*model.App, error) {
	app, err := r.k8sClient.App(ctx, name, team.String(), env)
	if err != nil {
		return nil, apierror.ErrAppNotFound
	}

	return app, nil
}

func (r *Resolver) App() gengql.AppResolver { return &appResolver{r} }

func (r *Resolver) AppUtilization() gengql.AppUtilizationResolver { return &appUtilizationResolver{r} }

type (
	appResolver            struct{ *Resolver }
	appUtilizationResolver struct{ *Resolver }
)
