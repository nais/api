package cost

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/cost/costsql"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"golang.org/x/exp/maps"
	"k8s.io/utils/ptr"
)

type Client interface {
	DailyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, fromDate, toDate time.Time) (*WorkloadCostPeriod, error)
	MonthlyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) (*WorkloadCostPeriod, error)
	DailyForTeamEnvironment(ctx context.Context, teamSlug slug.Slug, environmentName string, fromDate, toDate time.Time) (*TeamEnvironmentCostPeriod, error)
	DailyForTeam(ctx context.Context, teamSlug slug.Slug, fromDate, toDate time.Time, filter *TeamCostDailyFilter) (*TeamCostPeriod, error)
	MonthlySummaryForTeam(ctx context.Context, teamSlug slug.Slug) (*TeamCostMonthlySummary, error)
	MonthlyForService(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName, service string) (float32, error)
	MonthlySummaryForTenant(ctx context.Context) (*CostMonthlySummary, error)
}

type client struct{}

func (client) DailyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, fromDate, toDate time.Time) (*WorkloadCostPeriod, error) {
	rows, err := db(ctx).DailyCostForWorkload(ctx, costsql.DailyCostForWorkloadParams{
		FromDate:    pgtype.Date{Time: fromDate, Valid: true},
		ToDate:      pgtype.Date{Time: toDate, Valid: true},
		TeamSlug:    teamSlug,
		Environment: environmentName,
		AppLabel:    workloadName,
	})
	if err != nil {
		return nil, err
	}

	daily := make(map[pgtype.Date]*ServiceCostSeries)
	for _, row := range rows {
		if _, exists := daily[row.Date]; !exists {
			daily[row.Date] = &ServiceCostSeries{
				Date: scalar.NewDate(row.Date.Time),
			}
		}

		if row.Service != nil {
			daily[row.Date].Services = append(daily[row.Date].Services, &ServiceCostSample{
				Service: *row.Service,
				Cost:    float64(ptr.Deref(row.DailyCost, 0)),
			})
		}
	}

	ret := maps.Values(daily)
	slices.SortFunc(ret, func(a, b *ServiceCostSeries) int {
		return a.Date.Time().Compare(b.Date.Time())
	})

	return &WorkloadCostPeriod{
		Series: ret,
	}, nil
}

func (client) MonthlyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) (*WorkloadCostPeriod, error) {
	rows, err := db(ctx).MonthlyCostForWorkload(ctx, costsql.MonthlyCostForWorkloadParams{
		TeamSlug:    teamSlug,
		AppLabel:    workloadName,
		Environment: environmentName,
	})
	if err != nil {
		return nil, err
	}

	daily := make(map[pgtype.Date]*ServiceCostSeries)
	for _, row := range rows {
		if _, exists := daily[row.Month]; !exists {
			daily[row.Month] = &ServiceCostSeries{
				Date: scalar.NewDate(row.LastRecordedDate.Time),
			}
		}

		daily[row.Month].Services = append(daily[row.Month].Services, &ServiceCostSample{
			Service: row.Service,
			Cost:    float64(row.DailyCost),
		})
	}

	ret := maps.Values(daily)
	slices.SortFunc(ret, func(a, b *ServiceCostSeries) int {
		return b.Date.Time().Compare(a.Date.Time())
	})

	return &WorkloadCostPeriod{
		Series: ret,
	}, nil
}

func (client) DailyForTeamEnvironment(ctx context.Context, teamSlug slug.Slug, environmentName string, fromDate, toDate time.Time) (*TeamEnvironmentCostPeriod, error) {
	rows, err := db(ctx).DailyCostForTeamEnvironment(ctx, costsql.DailyCostForTeamEnvironmentParams{
		FromDate:    pgtype.Date{Time: fromDate, Valid: true},
		ToDate:      pgtype.Date{Time: toDate, Valid: true},
		TeamSlug:    teamSlug,
		Environment: environmentName,
	})
	if err != nil {
		return nil, err
	}

	daily := make(map[pgtype.Date]*WorkloadCostSeries)
	for _, row := range rows {
		if _, exists := daily[row.Date]; !exists {
			daily[row.Date] = &WorkloadCostSeries{
				Date: scalar.NewDate(row.Date.Time),
			}
		}

		if row.AppLabel != nil {
			daily[row.Date].Workloads = append(daily[row.Date].Workloads, &WorkloadCostSample{
				Cost:            float64(row.DailyCost),
				TeamSlug:        teamSlug,
				EnvironmentName: environmentName,
				WorkloadName:    *row.AppLabel,
			})
		}
	}

	ret := maps.Values(daily)
	slices.SortFunc(ret, func(a, b *WorkloadCostSeries) int {
		return a.Date.Time().Compare(b.Date.Time())
	})

	return &TeamEnvironmentCostPeriod{
		Series: ret,
	}, nil
}

func (client) DailyForTeam(ctx context.Context, teamSlug slug.Slug, fromDate, toDate time.Time, filter *TeamCostDailyFilter) (*TeamCostPeriod, error) {
	var services []string
	if filter != nil {
		services = filter.Services
	}

	rows, err := db(ctx).DailyCostForTeam(ctx, costsql.DailyCostForTeamParams{
		FromDate: pgtype.Date{Time: fromDate, Valid: true},
		ToDate:   pgtype.Date{Time: toDate, Valid: true},
		TeamSlug: teamSlug,
		Services: services,
	})
	if err != nil {
		return nil, err
	}

	daily := make(map[pgtype.Date]*ServiceCostSeries)
	for _, row := range rows {
		if _, exists := daily[row.Date]; !exists {
			daily[row.Date] = &ServiceCostSeries{
				Date: scalar.NewDate(row.Date.Time),
			}
		}

		if row.Service != nil {
			daily[row.Date].Services = append(daily[row.Date].Services, &ServiceCostSample{
				Service: *row.Service,
				Cost:    float64(row.Cost),
			})
		}
	}

	ret := maps.Values(daily)
	slices.SortFunc(ret, func(a, b *ServiceCostSeries) int {
		return b.Date.Time().Compare(a.Date.Time())
	})

	return &TeamCostPeriod{
		Series: ret,
	}, nil
}

func (client) MonthlySummaryForTenant(ctx context.Context) (*CostMonthlySummary, error) {
	rows, err := db(ctx).MonthlyCostForTenant(ctx)
	if err != nil {
		return nil, err
	}

	daily := make(map[pgtype.Date]*ServiceCostSeries)
	for _, row := range rows {
		if _, exists := daily[row.Month]; !exists {
			daily[row.Month] = &ServiceCostSeries{
				Date: scalar.NewDate(row.LastRecordedDate.Time),
			}
		}

		daily[row.Month].Services = append(daily[row.Month].Services, &ServiceCostSample{
			Service: row.Service,
			Cost:    float64(row.DailyCost),
		})
	}

	ret := maps.Values(daily)
	slices.SortFunc(ret, func(a, b *ServiceCostSeries) int {
		return a.Date.Time().Compare(b.Date.Time())
	})

	return &CostMonthlySummary{
		Series: ret,
	}, nil
}

func (client) MonthlySummaryForTeam(ctx context.Context, teamSlug slug.Slug) (*TeamCostMonthlySummary, error) {
	rows, err := db(ctx).MonthlyCostForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	ret := &TeamCostMonthlySummary{}

	for _, row := range rows {
		ret.Series = append(ret.Series, &TeamCostMonthlySample{
			Date: scalar.NewDate(row.LastRecordedDate.Time),
			Cost: float64(row.DailyCost),
		})
	}

	return ret, nil
}

func (client) MonthlyForService(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName, service string) (float32, error) {
	now := time.Now()

	to := pgtype.Date{Time: now, Valid: true}
	from := pgtype.Date{Time: now.AddDate(0, 0, -32), Valid: true}

	return db(ctx).CostForService(ctx, costsql.CostForServiceParams{
		FromDate:    from,
		ToDate:      to,
		TeamSlug:    teamSlug,
		Service:     service,
		AppLabel:    workloadName,
		Environment: environmentName,
	})
}
