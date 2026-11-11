package cost

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/cost/costsql"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type PostgresCost struct {
	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
	PostgresName    string    `json:"-"`
}

type PostgresCostPeriod struct {
	Series []*ServiceCostSeries `json:"series"`
}

func (p *PostgresCostPeriod) Sum() float64 {
	var sum float64
	for _, series := range p.Series {
		sum += series.Sum()
	}
	return sum
}

func DailyForPostgres(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName string, fromDate, toDate time.Time) (*PostgresCostPeriod, error) {
	return fromContext(ctx).client.DailyForPostgres(ctx, teamSlug, environmentName, postgresName, fromDate, toDate)
}

func MonthlyForPostgres(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName string) (*PostgresCostPeriod, error) {
	return fromContext(ctx).client.MonthlyForPostgres(ctx, teamSlug, environmentName, postgresName)
}

func (client) DailyForPostgres(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName string, fromDate, toDate time.Time) (*PostgresCostPeriod, error) {
	rows, err := db(ctx).DailyCostForPostgres(ctx, costsql.DailyCostForPostgresParams{
		TeamSlug:     teamSlug,
		Environment:  environmentName,
		PostgresName: postgresName,
		FromDate:     pgtype.Date{Time: fromDate, Valid: true},
		ToDate:       pgtype.Date{Time: toDate, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	samples := make([]postgresCostSample, 0, len(rows))
	for _, row := range rows {
		samples = append(samples, postgresCostSample{
			period:  row.Date.Time,
			date:    row.Date.Time,
			service: row.Service,
			cost:    float64(row.DailyCost),
		})
	}
	return newPostgresCostPeriod(samples, false), nil
}

func (client) MonthlyForPostgres(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName string) (*PostgresCostPeriod, error) {
	rows, err := db(ctx).MonthlyCostForPostgres(ctx, costsql.MonthlyCostForPostgresParams{
		TeamSlug:     teamSlug,
		Environment:  environmentName,
		PostgresName: postgresName,
	})
	if err != nil {
		return nil, err
	}
	samples := make([]postgresCostSample, 0, len(rows))
	for _, row := range rows {
		samples = append(samples, postgresCostSample{
			period:  row.Month.Time,
			date:    row.LastRecordedDate.Time,
			service: row.Service,
			cost:    float64(row.DailyCost),
		})
	}
	return newPostgresCostPeriod(samples, true), nil
}

type postgresCostSample struct {
	period  time.Time
	date    time.Time
	service string
	cost    float64
}

func newPostgresCostPeriod(samples []postgresCostSample, newestFirst bool) *PostgresCostPeriod {
	if len(samples) == 0 {
		return nil
	}
	periods := make(map[time.Time]*ServiceCostSeries)
	for _, sample := range samples {
		series, ok := periods[sample.period]
		if !ok {
			series = &ServiceCostSeries{Date: scalar.NewDate(sample.date)}
			periods[sample.period] = series
		} else if sample.date.After(series.Date.Time()) {
			series.Date = scalar.NewDate(sample.date)
		}
		series.Services = append(series.Services, &ServiceCostSample{Service: sample.service, Cost: sample.cost})
	}
	ret := &PostgresCostPeriod{Series: make([]*ServiceCostSeries, 0, len(periods))}
	for _, series := range periods {
		slices.SortFunc(series.Services, func(a, b *ServiceCostSample) int {
			return strings.Compare(a.Service, b.Service)
		})
		ret.Series = append(ret.Series, series)
	}
	slices.SortFunc(ret.Series, func(a, b *ServiceCostSeries) int {
		if newestFirst {
			return b.Date.Time().Compare(a.Date.Time())
		}
		return a.Date.Time().Compare(b.Date.Time())
	})
	return ret
}
