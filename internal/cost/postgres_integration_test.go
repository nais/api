//go:build integration_test

package cost

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/cost/costsql"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPostgresCostQueries(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("postgres_cost_test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Error(err)
		}
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	log := logrus.New()
	log.SetLevel(logrus.WarnLevel)
	pool, err := database.New(ctx, dsn, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `INSERT INTO teams (slug, purpose, slack_channel)
		VALUES ('team', 'Cost test', '#test'), ('other-team', 'Cost test', '#test')`); err != nil {
		t.Fatal(err)
	}

	date := func(value string) pgtype.Date { return pgtype.Date{Time: costDate(value), Valid: true} }
	row := func(team, env, service, name, app, day string, cost float32) costsql.CostUpsertParams {
		var serviceName *string
		if name != "" {
			serviceName = &name
		}
		return costsql.CostUpsertParams{
			Environment: &env, TeamSlug: slug.Slug(team), AppLabel: app, Service: service,
			ServiceName: serviceName, Date: date(day), DailyCost: cost,
		}
	}
	rows := []costsql.CostUpsertParams{
		row("team", "dev", PostgresService, "orders", "", "2026-02-28", 14),
		row("team", "dev", PostgresService, "orders", "", "2026-03-01", 0),
		row("team", "dev", PostgresService, "orders", "", "2026-03-02", 7),
		// Outside the 12-month window anchored to the latest imported month.
		row("team", "dev", PostgresService, "orders", "", "2025-03-31", 8),
		row("team", "dev", PostgresService, "orders", "", "2025-04-01", 6),
		// Must not leak into team/dev/orders.
		row("other-team", "dev", PostgresService, "orders", "", "2026-02-28", 100),
		row("team", "prod", PostgresService, "orders", "", "2026-02-28", 100),
		row("team", "dev", PostgresService, "other", "", "2026-02-28", 100),
		row("team", "dev", "Valkey", "orders", "", "2026-02-28", 100),
		row("team", "dev", "Compute Engine", "", "orders", "2026-02-28", 100),
	}
	q := costsql.New(pool)
	// Repeated imports must replace, not duplicate.
	for range 2 {
		q.CostUpsert(ctx, rows).Exec(func(i int, err error) {
			if err != nil {
				t.Errorf("upsert row %d: %v", i, err)
			}
		})
	}

	ctx = NewLoaderContext(ctx, pool)
	daily, err := DailyForPostgres(ctx, "team", "dev", "orders", costDate("2026-02-28"), costDate("2026-03-02"))
	if err != nil {
		t.Fatal(err)
	}
	if daily == nil || len(daily.Series) != 3 || daily.Sum() != 21 {
		t.Errorf("daily = %+v; want 3 days summing to 21", daily)
	}

	single, err := DailyForPostgres(ctx, "team", "dev", "orders", costDate("2026-03-01"), costDate("2026-03-01"))
	if err != nil || single == nil || len(single.Series) != 1 || single.Sum() != 0 {
		t.Errorf("single-day explicit zero = %+v, %v; want one zero sample", single, err)
	}

	missing, err := DailyForPostgres(ctx, "team", "dev", "missing", costDate("2026-02-28"), costDate("2026-03-02"))
	if err != nil || missing != nil {
		t.Errorf("missing Postgres = %+v, %v; want nil, nil", missing, err)
	}

	monthly, err := MonthlyForPostgres(ctx, "team", "dev", "orders")
	if err != nil {
		t.Fatal(err)
	}
	if monthly == nil || len(monthly.Series) != 3 || monthly.Sum() != 27 {
		t.Fatalf("monthly = %+v; want 3 months summing to 27", monthly)
	}
	if got := monthly.Series[0].Date.Time(); !got.Equal(costDate("2026-03-02")) {
		t.Errorf("latest monthly date = %v, want last observed date 2026-03-02", got)
	}
	if got := monthly.Series[2].Date.Time(); !got.Equal(costDate("2025-04-01")) {
		t.Errorf("oldest monthly date = %v, want 2025-04-01 (2025-03 excluded)", got)
	}
}
