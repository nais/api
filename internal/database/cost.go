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
	DailyEnvCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment *string, teamSlug slug.Slug) ([]*gensql.CostDailyTeam, error)
	LastCostDate(ctx context.Context) (pgtype.Date, error)
	MonthlyCostForApp(ctx context.Context, teamSlug slug.Slug, app string, environment string) ([]*gensql.CostMonthlyApp, error)
	MonthlyCostForTeam(ctx context.Context, teamSlug slug.Slug) ([]*gensql.CostMonthlyTeam, error)
}

var _ CostRepo = (*database)(nil)

func (d *database) CostUpsert(ctx context.Context, arg []gensql.CostUpsertParams) *gensql.CostUpsertBatchResults {
	return d.querier.CostUpsert(ctx, arg)
}

func (d *database) DailyCostForApp(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment string, teamSlug slug.Slug, app string) ([]*gensql.Cost, error) {
	return d.querier.DailyCostForApp(ctx, gensql.DailyCostForAppParams{
		FromDate:    fromDate,
		ToDate:      toDate,
		Environment: environment,
		TeamSlug:    teamSlug,
		App:         app,
	})
}

func (d *database) DailyCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, teamSlug slug.Slug) ([]*gensql.Cost, error) {
	return d.querier.DailyCostForTeam(ctx, gensql.DailyCostForTeamParams{
		FromDate: fromDate,
		ToDate:   toDate,
		TeamSlug: teamSlug,
	})
}

func (d *database) DailyEnvCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment *string, teamSlug slug.Slug) ([]*gensql.CostDailyTeam, error) {
	return d.querier.DailyEnvCostForTeam(ctx, gensql.DailyEnvCostForTeamParams{
		FromDate:    fromDate,
		ToDate:      toDate,
		Environment: environment,
		TeamSlug:    teamSlug,
	})
}

func (d *database) LastCostDate(ctx context.Context) (pgtype.Date, error) {
	return d.querier.LastCostDate(ctx)
}

func (d *database) MonthlyCostForApp(ctx context.Context, teamSlug slug.Slug, app string, environment string) ([]*gensql.CostMonthlyApp, error) {
	return d.querier.MonthlyCostForApp(ctx, gensql.MonthlyCostForAppParams{
		TeamSlug:    teamSlug,
		App:         app,
		Environment: environment,
	})
}

func (d *database) MonthlyCostForTeam(ctx context.Context, teamSlug slug.Slug) ([]*gensql.CostMonthlyTeam, error) {
	return d.querier.MonthlyCostForTeam(ctx, teamSlug)
}
