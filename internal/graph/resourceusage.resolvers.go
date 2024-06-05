package graph

import (
	"context"
	"errors"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func (r *queryResolver) ResourceUtilizationTrendForTeam(ctx context.Context, team slug.Slug) (*model.ResourceUtilizationTrend, error) {
	trend, err := r.resourceUsageClient.ResourceUtilizationTrendForTeam(ctx, team)
	if err != nil {
		return nil, err
	}

	if trend == nil {
		return &model.ResourceUtilizationTrend{}, nil
	}

	return trend, nil
}

func (r *queryResolver) CurrentResourceUtilizationForApp(ctx context.Context, env string, team slug.Slug, app string) (*model.CurrentResourceUtilization, error) {
	resp, err := r.resourceUsageClient.CurrentResourceUtilizationForApp(ctx, env, team, app)
	if errors.Is(err, pgx.ErrNoRows) {
		m := model.ResourceUtilization{
			Timestamp: time.Now(),
		}
		return &model.CurrentResourceUtilization{
			CPU:    m,
			Memory: m,
		}, nil
	} else if err != nil {
		return nil, err
	}
	return resp, nil
}

func (r *queryResolver) CurrentResourceUtilizationForTeam(ctx context.Context, team slug.Slug) (*model.CurrentResourceUtilization, error) {
	resp, err := r.resourceUsageClient.CurrentResourceUtilizationForTeam(ctx, team)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		m := model.ResourceUtilization{
			Timestamp: time.Now(),
		}
		return &model.CurrentResourceUtilization{
			CPU:    m,
			Memory: m,
		}, nil
	}

	return resp, nil
}

func (r *queryResolver) ResourceUtilizationOverageForTeam(ctx context.Context, team slug.Slug) (*model.ResourceUtilizationOverageForTeam, error) {
	return r.resourceUsageClient.ResourceUtilizationOverageForTeam(ctx, team)
}

func (r *queryResolver) ResourceUtilizationForTeam(ctx context.Context, team slug.Slug, from *scalar.Date, to *scalar.Date) ([]*model.ResourceUtilizationForEnv, error) {
	now := time.Now().Truncate(24 * time.Hour).UTC()
	if to == nil {
		d := scalar.NewDate(now)
		to = &d
	}

	if from == nil {
		d := scalar.NewDate(now.AddDate(0, 0, -7))
		from = &d
	}

	return r.resourceUsageClient.ResourceUtilizationForTeam(ctx, team, *from, *to)
}

func (r *queryResolver) ResourceUtilizationDateRangeForTeam(ctx context.Context, team slug.Slug) (*model.ResourceUtilizationDateRange, error) {
	return r.resourceUsageClient.ResourceUtilizationRangeForTeam(ctx, team)
}

func (r *queryResolver) ResourceUtilizationDateRangeForApp(ctx context.Context, env string, team slug.Slug, app string) (*model.ResourceUtilizationDateRange, error) {
	return r.resourceUsageClient.ResourceUtilizationRangeForApp(ctx, env, team, app)
}

func (r *queryResolver) ResourceUtilizationForApp(ctx context.Context, env string, team slug.Slug, app string, from *scalar.Date, to *scalar.Date) (*model.ResourceUtilizationForApp, error) {
	now := time.Now().Truncate(24 * time.Hour).UTC()
	if to == nil {
		d := scalar.NewDate(now)
		to = &d
	}

	if from == nil {
		d := scalar.NewDate(now.AddDate(0, 0, -7))
		from = &d
	}

	return r.resourceUsageClient.ResourceUtilizationForApp(ctx, env, team, app, *from, *to)
}
