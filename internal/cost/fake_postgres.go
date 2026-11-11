package cost

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/nais/api/internal/slug"
)

type postgresCostCacheKey struct {
	teamSlug        slug.Slug
	environmentName string
	postgresName    string
	from, to        time.Time
}

func (c *FakeClient) DailyForPostgres(_ context.Context, teamSlug slug.Slug, environmentName, postgresName string, from, to time.Time) (*PostgresCostPeriod, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	key := postgresCostCacheKey{teamSlug: teamSlug, environmentName: environmentName, postgresName: postgresName, from: from, to: to}
	if cached, ok := c.dailyForPostgresCache[key]; ok {
		return cached, nil
	}
	lastBillingDate := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -2)
	var samples []postgresCostSample
	for date := from; !date.After(to) && !date.After(lastBillingDate); date = date.AddDate(0, 0, 1) {
		if date.Before(startOfCost) {
			continue
		}
		samples = append(samples, postgresCostSample{period: date, date: date, service: "Compute Engine", cost: rand.Float64()})
	}
	period := newPostgresCostPeriod(samples, false)
	c.dailyForPostgresCache[key] = period
	return period, nil
}

func (c *FakeClient) MonthlyForPostgres(ctx context.Context, teamSlug slug.Slug, environmentName, postgresName string) (*PostgresCostPeriod, error) {
	lastBillingDate := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -2)
	month := time.Date(lastBillingDate.Year(), lastBillingDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	daily, err := c.DailyForPostgres(ctx, teamSlug, environmentName, postgresName, month.AddDate(0, -11, 0), lastBillingDate)
	if err != nil || daily == nil {
		return nil, err
	}
	months := make(map[time.Time]postgresCostSample)
	for _, series := range daily.Series {
		date := series.Date.Time()
		month := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
		sample := months[month]
		sample.period = month
		sample.date = date
		sample.service = "Compute Engine"
		sample.cost += series.Sum()
		months[month] = sample
	}
	samples := make([]postgresCostSample, 0, len(months))
	for _, sample := range months {
		samples = append(samples, sample)
	}
	return newPostgresCostPeriod(samples, true), nil
}
