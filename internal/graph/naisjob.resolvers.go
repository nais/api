package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) DeleteJob(ctx context.Context, name string, team slug.Slug, env string) (*model.DeleteJobResult, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return nil, err
	}

	if err := r.k8sClient.DeleteJob(ctx, name, team.String(), env); err != nil {
		return &model.DeleteJobResult{
			Deleted: false,
			Error:   ptr.To(err.Error()),
		}, nil
	}

	return &model.DeleteJobResult{
		Deleted: true,
	}, nil
}

func (r *naisJobResolver) Persistence(ctx context.Context, obj *model.NaisJob) ([]model.Persistence, error) {
	return r.k8sClient.Persistence(ctx, obj.WorkloadBase)
}

func (r *naisJobResolver) ImageDetails(ctx context.Context, obj *model.NaisJob) (*model.ImageDetails, error) {
	image, err := r.dependencyTrackClient.GetMetadataForImage(ctx, obj.Image)
	if err != nil {
		return nil, fmt.Errorf("getting metadata for image %q: %w", obj.Image, err)
	}

	return image, nil
}

func (r *naisJobResolver) Runs(ctx context.Context, obj *model.NaisJob) ([]*model.Run, error) {
	runs, err := r.k8sClient.Runs(ctx, obj.GQLVars.Team.String(), obj.Env.Name, obj.Name)
	if err != nil {
		return nil, err
	}
	return runs, nil
}

func (r *naisJobResolver) Manifest(ctx context.Context, obj *model.NaisJob) (string, error) {
	return r.k8sClient.NaisJobManifest(ctx, obj.Name, obj.GQLVars.Team.String(), obj.Env.Name)
}

func (r *naisJobResolver) Team(ctx context.Context, obj *model.NaisJob) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *naisJobResolver) Secrets(ctx context.Context, obj *model.NaisJob) ([]*model.Secret, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, obj.GQLVars.Team)
	if err != nil {
		return nil, err
	}

	return r.k8sClient.SecretsForNaisJob(ctx, obj)
}

func (r *queryResolver) Naisjob(ctx context.Context, name string, team slug.Slug, env string) (*model.NaisJob, error) {
	job, err := r.k8sClient.NaisJob(ctx, name, team.String(), env)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (r *Resolver) NaisJob() gengql.NaisJobResolver { return &naisJobResolver{r} }

type naisJobResolver struct{ *Resolver }
