package cost

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/cost/costsql"
	"github.com/nais/api/internal/v1/graphv1/scalar"
)

func ForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, fromDate, toDate time.Time) (*CostDaily, error) {
	rows, err := db(ctx).DailyCostForApp(ctx, costsql.DailyCostForAppParams{
		FromDate:    pgtype.Date{Time: fromDate, Valid: true},
		ToDate:      pgtype.Date{Time: toDate, Valid: true},
		Environment: environmentName,
		TeamSlug:    teamSlug,
		App:         workloadName,
	})
	if err != nil {
		return nil, err
	}

	costs, sum := dailyCostsFromDatabaseRows(from, to, rows)

	series := make([]*CostDailyEntry, 0)
	for costType, data := range costs {
		costTypeSum := 0.0
		for _, cost := range data {
			costTypeSum += cost.Cost
		}
		series = append(series, &model.CostSeries{
			CostType: costType,
			Sum:      costTypeSum,
			Data:     data,
		})
	}
}

func dailyCostsFromDatabaseRows(from, to scalar.Date, rows []*costsql.Cost) (SortedDailyCosts, float64) {
	sum := 0.0
	daily := CostDaily{}
	for _, row := range rows {
		if _, exists := daily[row.CostType]; !exists {
			daily[row.CostType] = make(map[scalar.Date]float64)
		}
		date := scalar.NewDate(row.Date.Time)
		if _, exists := daily[row.CostType][date]; !exists {
			daily[row.CostType][date] = 0.0
		}

		daily[row.CostType][date] += float64(row.DailyCost)
		sum += float64(row.DailyCost)
	}

	return normalizeDailyCosts(from, to, daily), sum
}

// normalizeDailyCosts will make sure all dates in the "from -> to" range are present in the returned map for all cost
// types. The dates will also be sorted in ascending order.
func normalizeDailyCosts(start, end time.Time, costs DailyCosts) SortedDailyCosts {
	sortedDailyCost := make(SortedDailyCosts)
	for k, daysInSeries := range costs {
		data := make([]*model.CostEntry, 0)
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			date := scalar.NewDate(day)
			cost := 0.0
			if c, exists := daysInSeries[date]; exists {
				cost = c
			}

			data = append(data, &model.CostEntry{
				Date: date,
				Cost: cost,
			})
		}

		sortedDailyCost[k] = data
	}

	return sortedDailyCost
}
