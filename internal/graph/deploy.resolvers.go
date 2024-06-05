package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/thirdparty/hookd"
)

func (r *queryResolver) Deployments(ctx context.Context, offset *int, limit *int) (*model.DeploymentList, error) {
	l := 100
	if limit != nil {
		l = *limit
	}
	deploys, err := r.hookdClient.Deployments(ctx, hookd.WithLimit(l), hookd.WithIgnoreTeams("nais-verification"))
	if err != nil {
		return nil, fmt.Errorf("getting deploys from Hookd: %w", err)
	}

	pagination := model.NewPagination(offset, limit)
	n, pi := model.PaginatedSlice(deploys, pagination)

	return &model.DeploymentList{
		Nodes:    deployToModel(n),
		PageInfo: pi,
	}, nil
}
