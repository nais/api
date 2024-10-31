package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

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

func (r *queryResolver) App(ctx context.Context, name string, team slug.Slug, env string) (*model.App, error) {
	app, err := r.k8sClient.App(ctx, name, team.String(), env)
	if err != nil {
		return nil, apierror.ErrAppNotFound
	}

	return app, nil
}

func (r *Resolver) App() gengql.AppResolver { return &appResolver{r} }

type appResolver struct{ *Resolver }
