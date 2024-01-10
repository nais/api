package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type CostRepo interface {
	CostUpsert(ctx context.Context, arg []gensql.CostUpsertParams) *gensql.CostUpsertBatchResults
	DailyCostForApp(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment string, teamSlug slug.Slug, app string) ([]*gensql.Cost, error)
	DailyCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, teamSlug slug.Slug) ([]*gensql.Cost, error)
	DailyEnvCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment *string, teamSlug slug.Slug) ([]*gensql.DailyEnvCostForTeamRow, error)
	LastCostDate(ctx context.Context) (pgtype.Date, error)
	MonthlyCostForApp(ctx context.Context, teamSlug slug.Slug, app string, environment string) ([]*gensql.MonthlyCostForAppRow, error)
	MonthlyCostForTeam(ctx context.Context, teamSlug slug.Slug) ([]*gensql.MonthlyCostForTeamRow, error)
}

func (d *database) CostUpsert(ctx context.Context, arg []gensql.CostUpsertParams) *gensql.CostUpsertBatchResults {
	return d.querier.CostUpsert(ctx, arg)
}

func (d *database) DailyCostForApp(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment string, teamSlug slug.Slug, app string) ([]*gensql.Cost, error) {
	return d.querier.DailyCostForApp(ctx, fromDate, toDate, environment, teamSlug, app)
}

func (d *database) DailyCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, teamSlug slug.Slug) ([]*gensql.Cost, error) {
	return d.querier.DailyCostForTeam(ctx, fromDate, toDate, teamSlug)
}

func (d *database) DailyEnvCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment *string, teamSlug slug.Slug) ([]*gensql.DailyEnvCostForTeamRow, error) {
	return d.querier.DailyEnvCostForTeam(ctx, fromDate, toDate, environment, teamSlug)
}

func (d *database) LastCostDate(ctx context.Context) (pgtype.Date, error) {
	return d.querier.LastCostDate(ctx)
}

func (d *database) MonthlyCostForApp(ctx context.Context, teamSlug slug.Slug, app string, environment string) ([]*gensql.MonthlyCostForAppRow, error) {
	return d.querier.MonthlyCostForApp(ctx, teamSlug, app, environment)
}

func (d *database) MonthlyCostForTeam(ctx context.Context, teamSlug slug.Slug) ([]*gensql.MonthlyCostForTeamRow, error) {
	return d.querier.MonthlyCostForTeam(ctx, teamSlug)
}
