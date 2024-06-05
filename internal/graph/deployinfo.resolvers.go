package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/thirdparty/hookd"
)

func (r *deployInfoResolver) History(ctx context.Context, obj *model.DeployInfo, offset *int, limit *int) (model.DeploymentResponse, error) {
	name := obj.GQLVars.App
	kind := "Application"
	if obj.GQLVars.Job != "" {
		kind = "Naisjob"
		name = obj.GQLVars.Job
	}

	deploys, err := r.hookdClient.Deployments(ctx, hookd.WithTeam(obj.GQLVars.Team.String()), hookd.WithCluster(obj.GQLVars.Env))
	if err != nil {
		return nil, fmt.Errorf("getting deploys from Hookd: %w", err)
	}

	deploys = filterDeploysByNameAndKind(deploys, name, kind)

	pagination := model.NewPagination(offset, limit)
	n, pi := model.PaginatedSlice(deploys, pagination)

	return &model.DeploymentList{
		Nodes:    deployToModel(n),
		PageInfo: pi,
	}, nil
}

func (r *Resolver) DeployInfo() gengql.DeployInfoResolver { return &deployInfoResolver{r} }

type deployInfoResolver struct{ *Resolver }
