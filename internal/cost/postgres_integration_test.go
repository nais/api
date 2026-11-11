//go:build integration_test

package cost

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/cost/costsql"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPostgresCostStorageAndQueries(t *testing.T) {
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
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	goose.SetBaseFS(os.DirFS("../database"))
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(ctx, db, "migrations", 82); err != nil {
		t.Fatal(err)
	}
	// The additive migration must preserve already imported application costs.
	if _, err := db.ExecContext(ctx, `INSERT INTO cost (environment, team_slug, app_label, service, date, daily_cost)
		VALUES ('dev', 'team', 'orders', 'Compute Engine', '2026-02-28', 100)`); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatal(err)
	}
	var preserved float32
	if err := db.QueryRowContext(ctx, "SELECT daily_cost FROM cost WHERE app_label = 'orders'").Scan(&preserved); err != nil {
		t.Fatal(err)
	}
	if preserved != 100 {
		t.Errorf("migration changed legacy cost to %v", preserved)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := costsql.New(pool)
	base := costsql.CostUpsertParams{
		Environment: new("dev"), TeamSlug: "team", PostgresName: new("orders"),
		Date: costDate("2026-02-28"), Service: "Compute Engine", DailyCost: 10,
	}
	rows := []costsql.CostUpsertParams{base}
	// Synthetic service dimensions prove that component costs are summed, not
	// merged with another Postgres or counted twice on repeated imports.
	pooler := base
	pooler.ServiceName, pooler.DailyCost = new("pooler"), 5
	rows = append(rows, pooler)
	storage := base
	storage.Service, storage.DailyCost = "Cloud Storage", 4
	rows = append(rows, storage)
	next := base
	next.Date, next.DailyCost = costDate("2026-03-01"), 7
	rows = append(rows, next)
	zero := base
	zero.Date, zero.DailyCost = costDate("2026-03-02"), 0
	rows = append(rows, zero)
	old := base
	old.Date, old.DailyCost = costDate("2025-03-31"), 8
	rows = append(rows, old)
	boundary := base
	boundary.Date, boundary.DailyCost = costDate("2025-04-01"), 6
	rows = append(rows, boundary)
	otherTeam := base
	otherTeam.TeamSlug, otherTeam.DailyCost = "other-team", 200
	rows = append(rows, otherTeam)
	otherEnvironment := base
	otherEnvironment.Environment, otherEnvironment.DailyCost = new("prod"), 100
	rows = append(rows, otherEnvironment)
	otherPostgres := base
	otherPostgres.PostgresName, otherPostgres.DailyCost = new("other"), 300
	rows = append(rows, otherPostgres)
	aiven := base
	aiven.PostgresName, aiven.Service, aiven.ServiceName, aiven.DailyCost = nil, "OpenSearch", new("search"), 15
	rows = append(rows, aiven)
	upsert := func(rows []costsql.CostUpsertParams) {
		t.Helper()
		q.CostUpsert(ctx, rows).Exec(func(i int, err error) {
			if err != nil {
				t.Errorf("upsert row %d: %v", i, err)
			}
		})
	}
	// Repeated imports replace costs without adding duplicate rows, including
	// null service_name and legacy rows with a null postgres_name.
	upsert(rows)
	upsert(rows)
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cost").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(rows)+1 {
		t.Errorf("repeat import produced %d rows, want %d", count, len(rows)+1)
	}
	// An actual correction updates the existing identity rather than inserting.
	correction := base
	correction.DailyCost = 11
	upsert([]costsql.CostUpsertParams{correction})
	upsert([]costsql.CostUpsertParams{base})

	dailyParams := costsql.DailyCostForPostgresParams{
		TeamSlug: "team", Environment: "dev", PostgresName: "orders",
		FromDate: costDate("2026-02-28"), ToDate: costDate("2026-03-02"),
	}
	daily, err := q.DailyCostForPostgres(ctx, dailyParams)
	if err != nil {
		t.Fatal(err)
	}
	var total float32
	for _, row := range daily {
		total += row.DailyCost
	}
	if len(daily) != 4 || total != 26 {
		t.Errorf("Postgres daily query: %d rows, total %v; want 4 rows, 26", len(daily), total)
	}
	// Both ends are inclusive, and an explicit zero is still a sample.
	dailyParams.FromDate, dailyParams.ToDate = costDate("2026-03-02"), costDate("2026-03-02")
	if rows, err := q.DailyCostForPostgres(ctx, dailyParams); err != nil || len(rows) != 1 || rows[0].DailyCost != 0 {
		t.Errorf("single-day zero sample = %v, %v", rows, err)
	}
	dailyParams.PostgresName = "missing"
	if rows, err := q.DailyCostForPostgres(ctx, dailyParams); err != nil || len(rows) != 0 {
		t.Errorf("missing resource = %v, %v", rows, err)
	}

	monthly, err := q.MonthlyCostForPostgres(ctx, costsql.MonthlyCostForPostgresParams{
		TeamSlug: "team", Environment: "dev", PostgresName: "orders",
	})
	if err != nil {
		t.Fatal(err)
	}
	total = 0
	for _, row := range monthly {
		total += row.DailyCost
		if row.Month.Time.Before(costDate("2025-04-01").Time) {
			t.Errorf("monthly query included month outside 12-month window: %v", row.Month)
		}
	}
	if len(monthly) != 4 || total != 32 || monthly[0].LastRecordedDate != costDate("2026-03-02") {
		t.Errorf("monthly query: %d service rows, total %v; want 4 rows, 32 and latest date March 2", len(monthly), total)
	}

	legacy, err := q.CostForService(ctx, costsql.CostForServiceParams{
		TeamSlug: "team", Environment: "dev", AppLabel: "orders", Service: "Compute Engine",
		FromDate: costDate("2026-02-28"), ToDate: costDate("2026-03-02"),
	})
	if err != nil || legacy != 100 {
		t.Errorf("same-named application cost = %v, %v; want 100, nil", legacy, err)
	}
	workloads, err := q.DailyCostForTeamEnvironment(ctx, costsql.DailyCostForTeamEnvironmentParams{
		TeamSlug: "team", Environment: "dev", FromDate: costDate("2026-02-28"), ToDate: costDate("2026-03-02"),
	})
	if err != nil {
		t.Fatal(err)
	}
	total = 0
	for _, row := range workloads {
		total += row.DailyCost
	}
	if total != 115 {
		t.Errorf("workload view includes Postgres-only costs: total %v, want legacy 115", total)
	}
	teamCosts, err := q.DailyCostForTeam(ctx, costsql.DailyCostForTeamParams{
		TeamSlug: "team", FromDate: costDate("2026-02-28"), ToDate: costDate("2026-03-02"),
	})
	if err != nil {
		t.Fatal(err)
	}
	total = 0
	for _, row := range teamCosts {
		total += row.Cost
	}
	if total != 541 {
		t.Errorf("team total = %v, want 541 including Postgres exactly once", total)
	}
	// Overall summaries continue to include every imported row exactly once.
	if err := q.RefreshCostMonthlyTeam(ctx); err != nil {
		t.Fatal(err)
	}
	if err := q.RefreshCostMonthlyTenant(ctx); err != nil {
		t.Fatal(err)
	}
	var sourceTotal, teamTotal, tenantTotal float64
	for query, dest := range map[string]*float64{
		"SELECT SUM(daily_cost) FROM cost":                &sourceTotal,
		"SELECT SUM(daily_cost) FROM cost_monthly_team":   &teamTotal,
		"SELECT SUM(daily_cost) FROM cost_monthly_tenant": &tenantTotal,
	} {
		if err := db.QueryRowContext(ctx, query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if sourceTotal != teamTotal || sourceTotal != tenantTotal {
		t.Errorf("totals differ: source=%v team=%v tenant=%v", sourceTotal, teamTotal, tenantTotal)
	}
	for _, statement := range []string{
		`INSERT INTO cost (environment, team_slug, app_label, service, date, daily_cost, postgres_name) VALUES ('dev', 'team', '', 'Compute Engine', '2026-03-03', 1, '')`,
		`INSERT INTO cost (environment, team_slug, app_label, service, date, daily_cost, postgres_name) VALUES ('dev', 'team', 'orders', 'Compute Engine', '2026-03-03', 1, 'orders')`,
	} {
		if _, err := db.ExecContext(ctx, statement); err == nil {
			t.Errorf("invalid mixed/empty Postgres identity was accepted: %s", statement)
		}
	}
}
