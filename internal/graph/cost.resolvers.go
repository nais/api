package graph

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func (r *queryResolver) DailyCostForApp(ctx context.Context, team slug.Slug, app string, env string, from scalar.Date, to scalar.Date) (*model.DailyCost, error) {
	err := ValidateDateInterval(from, to)
	if err != nil {
		return nil, err
	}

	fromDate, err := from.PgDate()
	if err != nil {
		return nil, err
	}

	toDate, err := to.PgDate()
	if err != nil {
		return nil, err
	}

	rows, err := r.database.DailyCostForApp(ctx, fromDate, toDate, env, team, app)
	if err != nil {
		return nil, fmt.Errorf("cost query: %w", err)
	}

	costs, sum := DailyCostsFromDatabaseRows(from, to, rows)
	series := make([]*model.CostSeries, 0)
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

	return &model.DailyCost{
		Sum:    sum,
		Series: series,
	}, nil
}

func (r *queryResolver) DailyCostForTeam(ctx context.Context, team slug.Slug, from scalar.Date, to scalar.Date) (*model.DailyCost, error) {
	err := ValidateDateInterval(from, to)
	if err != nil {
		return nil, err
	}

	fromDate, err := from.PgDate()
	if err != nil {
		return nil, err
	}

	toDate, err := to.PgDate()
	if err != nil {
		return nil, err
	}

	rows, err := r.database.DailyCostForTeam(ctx, fromDate, toDate, team)
	if err != nil {
		return nil, fmt.Errorf("cost query: %w", err)
	}

	costs, sum := DailyCostsForTeamFromDatabaseRows(from, to, rows)
	series := make([]*model.CostSeries, 0)

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

	return &model.DailyCost{
		Sum:    sum,
		Series: series,
	}, nil
}

func (r *queryResolver) MonthlyCost(ctx context.Context, filter model.MonthlyCostFilter) (*model.MonthlyCost, error) {
	if filter.App != "" && filter.Env != "" && filter.Team != "" {
		rows, err := r.database.MonthlyCostForApp(ctx, filter.Team, filter.App, filter.Env)
		if err != nil {
			return nil, err
		}
		sum := 0.0
		cost := make([]*model.CostEntry, len(rows))
		for idx, row := range rows {
			sum += float64(row.DailyCost)
			// make date variable equal last day in month of row.LastRecordedDate

			cost[idx] = &model.CostEntry{
				Date: scalar.NewDate(row.LastRecordedDate.Time),
				Cost: float64(row.DailyCost),
			}
		}
		return &model.MonthlyCost{
			Sum:  sum,
			Cost: cost,
		}, nil
	} else if filter.App == "" && filter.Env == "" && filter.Team != "" {
		rows, err := r.database.MonthlyCostForTeam(ctx, filter.Team)
		if err != nil {
			return nil, err
		}
		sum := 0.0
		cost := make([]*model.CostEntry, len(rows))
		for idx, row := range rows {
			sum += float64(row.DailyCost)
			// make date variable equal last day in month of row.LastRecordedDate

			cost[idx] = &model.CostEntry{
				Date: scalar.NewDate(row.LastRecordedDate.Time),
				Cost: float64(row.DailyCost),
			}
		}
		return &model.MonthlyCost{
			Sum:  sum,
			Cost: cost,
		}, nil
	}
	return nil, fmt.Errorf("not implemented")
}

func (r *queryResolver) EnvCost(ctx context.Context, filter model.EnvCostFilter) ([]*model.EnvCost, error) {
	err := ValidateDateInterval(filter.From, filter.To)
	if err != nil {
		return nil, err
	}

	fromDate, err := filter.From.PgDate()
	if err != nil {
		return nil, err
	}

	toDate, err := filter.To.PgDate()
	if err != nil {
		return nil, err
	}

	ret := []*model.EnvCost{}
	for clusterName, cluster := range r.clusters {
		if !cluster.GCP {
			continue
		}
		appsCost := make([]*model.AppCost, 0)
		rows, err := r.database.DailyEnvCostForTeam(ctx, fromDate, toDate, &clusterName, filter.Team)
		if err != nil {
			return nil, fmt.Errorf("cost query: %w", err)
		}

		costs, sum := DailyCostsForTeamPerEnvFromDatabaseRows(filter.From, filter.To, rows)

		for app, appCosts := range costs {
			appSum := 0.0
			for _, c := range appCosts {
				appSum += c.Cost
			}
			appsCost = append(appsCost, &model.AppCost{
				App:  app,
				Sum:  appSum,
				Cost: appCosts,
			})
		}

		sort.Slice(appsCost, func(i, j int) bool {
			return appsCost[i].Sum < appsCost[j].Sum
		})

		ret = append(ret, &model.EnvCost{
			Env:  clusterName,
			Apps: appsCost,
			Sum:  sum,
		})
	}

	slices.SortFunc(ret, func(a, b *model.EnvCost) int {
		return strings.Compare(a.Env, b.Env)
	})

	return ret, nil
}
