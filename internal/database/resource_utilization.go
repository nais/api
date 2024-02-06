package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type ResourceUtilizationRepo interface {
	AverageResourceUtilizationForTeam(ctx context.Context, teamSlug slug.Slug, resourceType gensql.ResourceType, timestamp pgtype.Timestamptz) (*gensql.AverageResourceUtilizationForTeamRow, error)
	MaxResourceUtilizationDate(ctx context.Context) (pgtype.Timestamptz, error)
	ResourceUtilizationForApp(ctx context.Context, arg gensql.ResourceUtilizationForAppParams) ([]*gensql.ResourceUtilizationMetric, error)
	ResourceUtilizationForTeam(ctx context.Context, environment string, teamSlug slug.Slug, resourceType gensql.ResourceType, start pgtype.Timestamptz, end pgtype.Timestamptz) ([]*gensql.ResourceUtilizationForTeamRow, error)
	ResourceUtilizationOverageForTeam(ctx context.Context, teamSlug slug.Slug, timestamp pgtype.Timestamptz, resourceType gensql.ResourceType) ([]*gensql.ResourceUtilizationOverageForTeamRow, error)
	ResourceUtilizationRangeForApp(ctx context.Context, environment string, teamSlug slug.Slug, app string) (*gensql.ResourceUtilizationRangeForAppRow, error)
	ResourceUtilizationRangeForTeam(ctx context.Context, teamSlug slug.Slug) (*gensql.ResourceUtilizationRangeForTeamRow, error)
	ResourceUtilizationUpsert(ctx context.Context, arg []gensql.ResourceUtilizationUpsertParams) *gensql.ResourceUtilizationUpsertBatchResults
	SpecificResourceUtilizationForApp(ctx context.Context, environment string, teamSlug slug.Slug, app string, resourceType gensql.ResourceType, timestamp pgtype.Timestamptz) (*gensql.SpecificResourceUtilizationForAppRow, error)
	SpecificResourceUtilizationForTeam(ctx context.Context, teamSlug slug.Slug, resourceType gensql.ResourceType, timestamp pgtype.Timestamptz) (*gensql.SpecificResourceUtilizationForTeamRow, error)
}

func (d *database) ResourceUtilizationUpsert(ctx context.Context, arg []gensql.ResourceUtilizationUpsertParams) *gensql.ResourceUtilizationUpsertBatchResults {
	return d.querier.ResourceUtilizationUpsert(ctx, arg)
}

func (d *database) AverageResourceUtilizationForTeam(ctx context.Context, teamSlug slug.Slug, resourceType gensql.ResourceType, timestamp pgtype.Timestamptz) (*gensql.AverageResourceUtilizationForTeamRow, error) {
	return d.querier.AverageResourceUtilizationForTeam(ctx, gensql.AverageResourceUtilizationForTeamParams{
		TeamSlug:     teamSlug,
		ResourceType: resourceType,
		Timestamp:    timestamp,
	})
}

func (d *database) MaxResourceUtilizationDate(ctx context.Context) (pgtype.Timestamptz, error) {
	return d.querier.MaxResourceUtilizationDate(ctx)
}

func (d *database) ResourceUtilizationForApp(ctx context.Context, arg gensql.ResourceUtilizationForAppParams) ([]*gensql.ResourceUtilizationMetric, error) {
	return d.querier.ResourceUtilizationForApp(ctx, arg)
}

func (d *database) ResourceUtilizationForTeam(ctx context.Context, environment string, teamSlug slug.Slug, resourceType gensql.ResourceType, start, end pgtype.Timestamptz) ([]*gensql.ResourceUtilizationForTeamRow, error) {
	return d.querier.ResourceUtilizationForTeam(ctx, gensql.ResourceUtilizationForTeamParams{
		Environment:  environment,
		TeamSlug:     teamSlug,
		ResourceType: resourceType,
		Start:        start,
		End:          end,
	})
}

func (d *database) ResourceUtilizationOverageForTeam(ctx context.Context, teamSlug slug.Slug, timestamp pgtype.Timestamptz, resourceType gensql.ResourceType) ([]*gensql.ResourceUtilizationOverageForTeamRow, error) {
	return d.querier.ResourceUtilizationOverageForTeam(ctx, gensql.ResourceUtilizationOverageForTeamParams{
		TeamSlug:     teamSlug,
		Timestamp:    timestamp,
		ResourceType: resourceType,
	})
}

func (d *database) ResourceUtilizationRangeForApp(ctx context.Context, environment string, teamSlug slug.Slug, app string) (*gensql.ResourceUtilizationRangeForAppRow, error) {
	return d.querier.ResourceUtilizationRangeForApp(ctx, gensql.ResourceUtilizationRangeForAppParams{
		Environment: environment,
		TeamSlug:    teamSlug,
		App:         app,
	})
}

func (d *database) ResourceUtilizationRangeForTeam(ctx context.Context, teamSlug slug.Slug) (*gensql.ResourceUtilizationRangeForTeamRow, error) {
	return d.querier.ResourceUtilizationRangeForTeam(ctx, teamSlug)
}

func (d *database) SpecificResourceUtilizationForApp(ctx context.Context, environment string, teamSlug slug.Slug, app string, resourceType gensql.ResourceType, timestamp pgtype.Timestamptz) (*gensql.SpecificResourceUtilizationForAppRow, error) {
	return d.querier.SpecificResourceUtilizationForApp(ctx, gensql.SpecificResourceUtilizationForAppParams{
		Environment:  environment,
		TeamSlug:     teamSlug,
		App:          app,
		ResourceType: resourceType,
		Timestamp:    timestamp,
	})
}

func (d *database) SpecificResourceUtilizationForTeam(ctx context.Context, teamSlug slug.Slug, resourceType gensql.ResourceType, timestamp pgtype.Timestamptz) (*gensql.SpecificResourceUtilizationForTeamRow, error) {
	return d.querier.SpecificResourceUtilizationForTeam(ctx, gensql.SpecificResourceUtilizationForTeamParams{
		TeamSlug:     teamSlug,
		ResourceType: resourceType,
		Timestamp:    timestamp,
	})
}
