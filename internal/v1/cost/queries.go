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

	daily := make(map[pgtype.Date]*WorkloadCostSeries)
	for _, row := range rows {
		if _, exists := daily[row.Date]; !exists {
			daily[row.Date] = &WorkloadCostSeries{
				Date: scalar.NewDate(row.Date.Time),
			}
		}

		if row.Service != nil {
			daily[row.Date].Services = append(daily[row.Date].Services, &WorkloadCostService{
				Service: *row.Service,
				Cost:    float64(ptr.Deref(row.DailyCost, 0)),
			})
		}
	}

	ret := maps.Values(daily)
	slices.SortFunc(ret, func(a, b *WorkloadCostSeries) int {
		return a.Date.Time().Compare(b.Date.Time())
	})

	return &WorkloadCostPeriod{
		Series: ret,
	}, nil
}
