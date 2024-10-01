package cost

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/cost/costsql"
	"github.com/nais/api/internal/v1/graphv1/scalar"
	"golang.org/x/exp/maps"
	"k8s.io/utils/ptr"
)

func DailyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, fromDate, toDate time.Time) (*WorkloadCostPeriod, error) {
	rows, err := db(ctx).DailyCostForWorkload(ctx, costsql.DailyCostForWorkloadParams{
		FromDate:    pgtype.Date{Time: fromDate, Valid: true},
		ToDate:      pgtype.Date{Time: toDate, Valid: true},
		TeamSlug:    teamSlug,
		Environment: environmentName,
		Workload:    workloadName,
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
			daily[row.Date].Services = append(daily[row.Date].Services, &ServiceCost{
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

func MonthlyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) (*WorkloadCostPeriod, error) {
	rows, err := db(ctx).MonthlyCostForWorkload(ctx, costsql.MonthlyCostForWorkloadParams{
		TeamSlug:    teamSlug,
		Workload:    workloadName,
		Environment: environmentName,
	})
	if err != nil {
		return nil, err
	}

	daily := make(map[pgtype.Date]*ServiceCostSeries)
	for _, row := range rows {
		if _, exists := daily[row.Month]; !exists {
			daily[row.Month] = &ServiceCostSeries{
				Date: scalar.NewDate(row.Month.Time),
			}
		}

		daily[row.Month].Services = append(daily[row.Month].Services, &ServiceCost{
			Service: row.Service,
			Cost:    float64(row.DailyCost),
		})
	}

	ret := maps.Values(daily)
	slices.SortFunc(ret, func(a, b *ServiceCostSeries) int {
		return a.Date.Time().Compare(b.Date.Time())
	})

	return &WorkloadCostPeriod{
		Series: ret,
	}, nil
}

func DailyForTeam(ctx context.Context, teamSlug slug.Slug, fromDate, toDate time.Time) (*TeamCostPeriod, error) {
	rows, err := db(ctx).DailyCostForTeam(ctx, costsql.DailyCostForTeamParams{
		FromDate: pgtype.Date{Time: fromDate, Valid: true},
		ToDate:   pgtype.Date{Time: toDate, Valid: true},
		TeamSlug: teamSlug,
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
			daily[row.Date].Services = append(daily[row.Date].Services, &ServiceCost{
				Service: *row.Service,
				Cost:    float64(row.Cost),
			})
		}
	}

	ret := maps.Values(daily)
	slices.SortFunc(ret, func(a, b *ServiceCostSeries) int {
		return a.Date.Time().Compare(b.Date.Time())
	})

	return &TeamCostPeriod{
		Series: ret,
	}, nil
}

func MonthlySummaryForTeam(ctx context.Context, teamSlug slug.Slug) (*TeamCostMonthlySummary, error) {
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

func MonthlyCostForService(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName, costType string) (float32, error) {
	now := time.Now()

	to := pgtype.Date{Time: now, Valid: true}
	from := pgtype.Date{Time: now.AddDate(0, 0, -32), Valid: true}

	return db(ctx).CostForInstance(ctx, costsql.CostForInstanceParams{
		FromDate:    from,
		ToDate:      to,
		TeamSlug:    teamSlug,
		CostType:    costType,
		Workload:    workloadName,
		Environment: environmentName,
	})
}
