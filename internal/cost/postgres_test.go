package cost

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/cost/costsql"
	"github.com/nais/api/internal/graph/scalar"
)

func costDate(value string) pgtype.Date {
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return pgtype.Date{Time: date, Valid: true}
}

var compareCostDates = cmp.Comparer(func(a, b scalar.Date) bool { return a.Time().Equal(b.Time()) })

func TestDailyForPostgres(t *testing.T) {
	from, to := costDate("2026-02-28"), costDate("2026-03-02")
	queryErr := errors.New("cost query failed")
	for _, tt := range []struct {
		name string
		rows []*costsql.DailyCostForPostgresRow
		err  error
		want *PostgresCostPeriod
	}{
		{name: "no samples is unknown"},
		{name: "query error is propagated", err: queryErr},
		{
			name: "explicit zero is recorded data",
			rows: []*costsql.DailyCostForPostgresRow{{Date: from, Service: "Compute Engine", DailyCost: 0}},
			want: &PostgresCostPeriod{Series: []*ServiceCostSeries{{Date: scalar.NewDate(from.Time), Services: []*ServiceCostSample{{Service: "Compute Engine", Cost: 0}}}}},
		},
		{
			name: "services sum without filling missing dates",
			rows: []*costsql.DailyCostForPostgresRow{
				{Date: to, Service: "Compute Engine", DailyCost: 7},
				{Date: from, Service: "Compute Engine", DailyCost: 10},
				{Date: from, Service: "Cloud Storage", DailyCost: 4},
			},
			want: &PostgresCostPeriod{Series: []*ServiceCostSeries{
				{Date: scalar.NewDate(from.Time), Services: []*ServiceCostSample{{Service: "Cloud Storage", Cost: 4}, {Service: "Compute Engine", Cost: 10}}},
				{Date: scalar.NewDate(to.Time), Services: []*ServiceCostSample{{Service: "Compute Engine", Cost: 7}}},
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			q := costsql.NewMockQuerier(t)
			ctx := context.WithValue(context.Background(), loadersKey, &loaders{internalQuerier: q})
			q.EXPECT().DailyCostForPostgres(ctx, costsql.DailyCostForPostgresParams{
				TeamSlug: "team", Environment: "dev", PostgresName: "orders", FromDate: from, ToDate: to,
			}).Return(tt.rows, tt.err).Once()
			got, err := (client{}).DailyForPostgres(ctx, "team", "dev", "orders", from.Time, to.Time)
			if !errors.Is(err, tt.err) {
				t.Fatalf("error = %v, want %v", err, tt.err)
			}
			if diff := cmp.Diff(tt.want, got, compareCostDates); diff != "" {
				t.Errorf("period (-want +got):\n%s", diff)
			}
			if got != nil && got.Sum() != tt.want.Sum() {
				t.Errorf("sum = %v, want %v", got.Sum(), tt.want.Sum())
			}
		})
	}
}

func TestMonthlyForPostgres(t *testing.T) {
	q := costsql.NewMockQuerier(t)
	ctx := context.WithValue(context.Background(), loadersKey, &loaders{internalQuerier: q})
	params := costsql.MonthlyCostForPostgresParams{TeamSlug: "team", Environment: "prod", PostgresName: "orders"}
	q.EXPECT().MonthlyCostForPostgres(ctx, params).Return([]*costsql.MonthlyCostForPostgresRow{
		{Month: costDate("2026-02-01"), LastRecordedDate: costDate("2026-02-20"), Service: "Compute Engine", DailyCost: 10},
		{Month: costDate("2026-03-01"), LastRecordedDate: costDate("2026-03-02"), Service: "Compute Engine", DailyCost: 7},
		{Month: costDate("2026-02-01"), LastRecordedDate: costDate("2026-02-28"), Service: "Cloud Storage", DailyCost: 4},
	}, nil).Once()
	got, err := (client{}).MonthlyForPostgres(ctx, "team", "prod", "orders")
	if err != nil {
		t.Fatal(err)
	}
	want := &PostgresCostPeriod{Series: []*ServiceCostSeries{
		{Date: scalar.NewDate(costDate("2026-03-02").Time), Services: []*ServiceCostSample{{Service: "Compute Engine", Cost: 7}}},
		{Date: scalar.NewDate(costDate("2026-02-28").Time), Services: []*ServiceCostSample{{Service: "Cloud Storage", Cost: 4}, {Service: "Compute Engine", Cost: 10}}},
	}}
	if diff := cmp.Diff(want, got, compareCostDates); diff != "" {
		t.Errorf("period (-want +got):\n%s", diff)
	}
	if got.Sum() != 21 {
		t.Errorf("sum = %v, want 21", got.Sum())
	}

	q.EXPECT().MonthlyCostForPostgres(ctx, params).Return(nil, nil).Once()
	if got, err := (client{}).MonthlyForPostgres(ctx, "team", "prod", "orders"); got != nil || err != nil {
		t.Errorf("missing monthly data = %v, %v; want nil, nil", got, err)
	}
	queryErr := errors.New("query failed")
	q.EXPECT().MonthlyCostForPostgres(ctx, params).Return(nil, queryErr).Once()
	if got, err := (client{}).MonthlyForPostgres(ctx, "team", "prod", "orders"); got != nil || !errors.Is(err, queryErr) {
		t.Errorf("monthly query failure = %v, %v; want nil, %v", got, err, queryErr)
	}
}

func TestFakePostgresCosts(t *testing.T) {
	fake := NewFakeClient()
	ctx := context.Background()
	from, to := costDate("2024-02-28").Time, costDate("2024-03-01").Time
	first, err := fake.DailyForPostgres(ctx, "team", "dev", "orders", from, to)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fake.DailyForPostgres(ctx, "team", "dev", "orders", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first.Series) != 3 {
		t.Error("fake did not cache the inclusive leap-day series")
	}
	for _, key := range []postgresCostCacheKey{
		{teamSlug: "other-team", environmentName: "dev", postgresName: "orders"},
		{teamSlug: "team", environmentName: "prod", postgresName: "orders"},
		{teamSlug: "team", environmentName: "dev", postgresName: "other"},
	} {
		other, err := fake.DailyForPostgres(ctx, key.teamSlug, key.environmentName, key.postgresName, from, to)
		if err != nil {
			t.Fatal(err)
		}
		if other == first {
			t.Errorf("cache merged distinct identity %+v", key)
		}
	}
	beforeData := costDate("2023-01-01").Time
	if got, err := fake.DailyForPostgres(ctx, "team", "dev", "orders", beforeData, beforeData); got != nil || err != nil {
		t.Errorf("fake missing data = %v, %v; want nil, nil", got, err)
	}
	monthly, err := fake.MonthlyForPostgres(ctx, "team", "dev", "orders")
	if err != nil {
		t.Fatal(err)
	}
	if monthly == nil || len(monthly.Series) != 12 {
		t.Fatalf("fake monthly series = %v, want 12 months", monthly)
	}
	for i := 1; i < len(monthly.Series); i++ {
		if !monthly.Series[i].Date.Time().Before(monthly.Series[i-1].Date.Time()) {
			t.Error("fake monthly series is not newest first")
		}
	}
}
