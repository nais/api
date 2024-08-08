package graph

import (
	"context"
	"time"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

const MaxDataPoints = 1000

func (r *queryResolver) ResourceUtilizationForAppV2(ctx context.Context, env string, team slug.Slug, app string, start time.Time, end time.Time, step int, resourceType model.ResourceTypeV2) ([]*model.ResourceUtilizationV2, error) {
	dpsRequested := ((int(end.Unix()) - int(start.Unix())) / step)
	if dpsRequested > MaxDataPoints {
		return nil, apierror.Errorf("maximum datapoints exceeded. Maximum allowed is %d, you requested %d", MaxDataPoints, dpsRequested)
	}

	return r.resourceUsageClientV2.ResourceUtilizationForApp(ctx, env, team, app, resourceType, start, end, step)
}
