package cost

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/cost/costsql"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

// PostgresService is the billing service for Postgres costs. Rows have no app
// label; service_name identifies the Postgres, like Aiven services.
const PostgresService = "CloudNativePG"

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
	rows, err := db(ctx).DailyCostForService(ctx, costsql.DailyCostForServiceParams{
		TeamSlug:    teamSlug,
		Environment: environmentName,
		Service:     PostgresService,
		ServiceName: postgresName,
		FromDate:    pgtype.Date{Time: fromDate, Valid: true},
		ToDate:      pgtype.Date{Time: toDate, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	samples := make([]postgresCostSample, 0, len(rows))
	for _, row := range rows {
		samples = append(samples, postgresCostSample{date: row.Date.Time, cost: float64(row.DailyCost)})
	}
	return newPostgresCostPeriod(samples, false), nil
}

func (client) MonthlyForPostgres(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName string) (*PostgresCostPeriod, error) {
	rows, err := db(ctx).MonthlyCostForService(ctx, costsql.MonthlyCostForServiceParams{
		TeamSlug:    teamSlug,
		Environment: environmentName,
		Service:     PostgresService,
		ServiceName: postgresName,
	})
	if err != nil {
		return nil, err
	}
	samples := make([]postgresCostSample, 0, len(rows))
	for _, row := range rows {
		samples = append(samples, postgresCostSample{date: row.LastRecordedDate.Time, cost: float64(row.DailyCost)})
	}
	return newPostgresCostPeriod(samples, true), nil
}

type postgresCostSample struct {
	date time.Time
	cost float64
}

// newPostgresCostPeriod returns nil when there are no samples, so missing data
// is not presented as a confirmed zero cost.
func newPostgresCostPeriod(samples []postgresCostSample, newestFirst bool) *PostgresCostPeriod {
	if len(samples) == 0 {
		return nil
	}
	ret := &PostgresCostPeriod{Series: make([]*ServiceCostSeries, 0, len(samples))}
	for _, sample := range samples {
		ret.Series = append(ret.Series, &ServiceCostSeries{
			Date:     scalar.NewDate(sample.date),
			Services: []*ServiceCostSample{{Service: PostgresService, Cost: sample.cost}},
		})
	}
	slices.SortFunc(ret.Series, func(a, b *ServiceCostSeries) int {
		if newestFirst {
			return b.Date.Time().Compare(a.Date.Time())
		}
		return a.Date.Time().Compare(b.Date.Time())
	})
	return ret
}
