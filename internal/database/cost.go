package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type CostRepo interface {
	CostForInstance(ctx context.Context, costType string, fromDate, toDate pgtype.Date, teamSlug slug.Slug, appName, environment string) (float32, error)
	CostRefresh(ctx context.Context) error
	CostUpsert(ctx context.Context, arg []gensql.CostUpsertParams) *gensql.CostUpsertBatchResults
	CurrentSqlInstancesCostForTeam(ctx context.Context, teamSlug slug.Slug) (float32, error)
	DailyCostForApp(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment string, teamSlug slug.Slug, app string) ([]*gensql.Cost, error)
	DailyCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, teamSlug slug.Slug) ([]*gensql.Cost, error)
	DailyEnvCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment *string, teamSlug slug.Slug) ([]*gensql.DailyEnvCostForTeamRow, error)
	LastCostDate(ctx context.Context) (pgtype.Date, error)
	MonthlyCostForApp(ctx context.Context, teamSlug slug.Slug, app string, environment string) ([]*gensql.MonthlyCostForAppRow, error)
	MonthlyCostForTeam(ctx context.Context, teamSlug slug.Slug) ([]*gensql.CostMonthlyTeam, error)
}

var _ CostRepo = (*database)(nil)

func (d *database) CostUpsert(ctx context.Context, arg []gensql.CostUpsertParams) *gensql.CostUpsertBatchResults {
	return d.querier.CostUpsert(ctx, arg)
}

func (d *database) CostForInstance(ctx context.Context, costType string, fromDate pgtype.Date, toDate pgtype.Date, teamSlug slug.Slug, appName, environment string) (float32, error) {
	cost, err := d.querier.CostForInstance(ctx, gensql.CostForInstanceParams{
		FromDate:    fromDate,
		ToDate:      toDate,
		TeamSlug:    teamSlug,
		AppName:     appName,
		Environment: environment,
		CostType:    costType,
	})
	if err != nil {
		return 0, err
	}

	return cost, nil
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

func (d *database) DailyEnvCostForTeam(ctx context.Context, fromDate pgtype.Date, toDate pgtype.Date, environment *string, teamSlug slug.Slug) ([]*gensql.DailyEnvCostForTeamRow, error) {
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

func (d *database) MonthlyCostForApp(ctx context.Context, teamSlug slug.Slug, app string, environment string) ([]*gensql.MonthlyCostForAppRow, error) {
	return d.querier.MonthlyCostForApp(ctx, gensql.MonthlyCostForAppParams{
		TeamSlug:    teamSlug,
		App:         app,
		Environment: environment,
	})
}

func (d *database) MonthlyCostForTeam(ctx context.Context, teamSlug slug.Slug) ([]*gensql.CostMonthlyTeam, error) {
	return d.querier.MonthlyCostForTeam(ctx, teamSlug)
}

func (d *database) CurrentSqlInstancesCostForTeam(ctx context.Context, teamSlug slug.Slug) (float32, error) {
	now := time.Now()
	var from, to pgtype.Date

	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -32)) // we don't have cost for today or yesterday

	return d.querier.CurrentSqlInstancesCostForTeam(ctx, gensql.CurrentSqlInstancesCostForTeamParams{
		FromDate: from,
		ToDate:   to,
		TeamSlug: teamSlug,
	})
}

func (d *database) CostRefresh(ctx context.Context) error {
	return d.querier.RefreshCostMonthlyTeam(ctx)
}
