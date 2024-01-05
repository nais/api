package graph

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

type (
	dailyCosts       map[string]map[scalar.Date]float64
	sortedDailyCosts map[string][]*model.CostEntry
)

// DailyCostsFromDatabaseRows will convert a slice of cost rows from the database to a sortedDailyCosts map.
func DailyCostsFromDatabaseRows(from, to scalar.Date, rows []*gensql.Cost) (sortedDailyCosts, float64) {
	sum := 0.0
	daily := dailyCosts{}
	for _, row := range rows {
		if _, exists := daily[row.CostType]; !exists {
			daily[row.CostType] = make(map[scalar.Date]float64)
		}
		daily[row.CostType][scalar.NewDate(row.Date.Time)] = float64(row.DailyCost)
		sum += float64(row.DailyCost)
	}

	return normalizeDailyCosts(from, to, daily), sum
}

func DailyCostsForTeamFromDatabaseRows(from, to scalar.Date, rows []*gensql.Cost) (sortedDailyCosts, float64) {
	sum := 0.0
	daily := dailyCosts{}
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

func DailyCostsForTeamPerEnvFromDatabaseRows(from, to scalar.Date, rows []*gensql.DailyEnvCostForTeamRow) (sortedDailyCosts, float64) {
	sum := 0.0
	daily := dailyCosts{}
	for _, row := range rows {
		if row.App == "" {
			continue
		}
		if _, exists := daily[row.App]; !exists {
			daily[row.App] = make(map[scalar.Date]float64)
		}
		daily[row.App][scalar.NewDate(row.Date.Time)] = float64(row.DailyCost)
		sum += float64(row.DailyCost)
	}

	return normalizeDailyCosts(from, to, daily), sum
}

// normalizeDailyCosts will make sure all dates in the "from -> to" range are present in the returned map for all cost
// types. The dates will also be sorted in ascending order.
func normalizeDailyCosts(from, to scalar.Date, costs dailyCosts) sortedDailyCosts {
	start, _ := from.Time()
	end, _ := to.Time()
	sortedDailyCost := make(sortedDailyCosts)
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

// ValidateDateInterval will validate a from => to date interval used for querying costs.
func ValidateDateInterval(from, to scalar.Date) error {
	today := scalar.NewDate(time.Now())
	if from > to {
		return fmt.Errorf("from date cannot be after to date")
	} else if to > today {
		return fmt.Errorf("to date cannot be in the future")
	}

	return nil
}
