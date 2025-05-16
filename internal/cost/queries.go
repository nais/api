package cost

import (
	"context"
	"time"

	"github.com/nais/api/internal/slug"
)

func DailyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, fromDate, toDate time.Time) (*WorkloadCostPeriod, error) {
	return fromContext(ctx).client.DailyForWorkload(ctx, teamSlug, environmentName, workloadName, fromDate, toDate)
}

func MonthlyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) (*WorkloadCostPeriod, error) {
	return fromContext(ctx).client.MonthlyForWorkload(ctx, teamSlug, environmentName, workloadName)
}

func DailyForTeamEnvironment(ctx context.Context, teamSlug slug.Slug, environmentName string, fromDate, toDate time.Time) (*TeamEnvironmentCostPeriod, error) {
	return fromContext(ctx).client.DailyForTeamEnvironment(ctx, teamSlug, environmentName, fromDate, toDate)
}

func DailyForTeam(ctx context.Context, teamSlug slug.Slug, fromDate, toDate time.Time, filter *TeamCostDailyFilter) (*TeamCostPeriod, error) {
	return fromContext(ctx).client.DailyForTeam(ctx, teamSlug, fromDate, toDate, filter)
}

func MonthlySummaryForTeam(ctx context.Context, teamSlug slug.Slug) (*TeamCostMonthlySummary, error) {
	return fromContext(ctx).client.MonthlySummaryForTeam(ctx, teamSlug)
}

func MonthlyForService(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName, service string) (float32, error) {
	return fromContext(ctx).client.MonthlyForService(ctx, teamSlug, environmentName, workloadName, service)
}

func MonthlySummaryForTenant(ctx context.Context) (*TenantCostMonthlySummary, error) {
	return fromContext(ctx).client.MonthlySummaryForTenant(ctx)
}
