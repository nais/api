package cost

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/scalar"
)

func costDate(value string) time.Time {
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return date
}

var compareCostDates = cmp.Comparer(func(a, b scalar.Date) bool { return a.Time().Equal(b.Time()) })

func TestNewPostgresCostPeriod(t *testing.T) {
	series := func(date string, cost float64) *ServiceCostSeries {
		return &ServiceCostSeries{
			Date:     scalar.NewDate(costDate(date)),
			Services: []*ServiceCostSample{{Service: PostgresService, Cost: cost}},
		}
	}
	samples := []postgresCostSample{
		{date: costDate("2026-03-02"), cost: 7},
		{date: costDate("2026-02-28"), cost: 14},
		{date: costDate("2026-03-01"), cost: 0},
	}

	for _, tt := range []struct {
		name        string
		samples     []postgresCostSample
		newestFirst bool
		want        *PostgresCostPeriod
	}{
		{name: "no samples is unknown, not zero"},
		{
			name:    "oldest first, explicit zero kept, missing dates not filled",
			samples: samples,
			want:    &PostgresCostPeriod{Series: []*ServiceCostSeries{series("2026-02-28", 14), series("2026-03-01", 0), series("2026-03-02", 7)}},
		},
		{
			name:        "newest first",
			samples:     samples,
			newestFirst: true,
			want:        &PostgresCostPeriod{Series: []*ServiceCostSeries{series("2026-03-02", 7), series("2026-03-01", 0), series("2026-02-28", 14)}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := newPostgresCostPeriod(tt.samples, tt.newestFirst)
			if diff := cmp.Diff(tt.want, got, compareCostDates); diff != "" {
				t.Errorf("period (-want +got):\n%s", diff)
			}
			if got != nil && got.Sum() != 21 {
				t.Errorf("sum = %v, want 21", got.Sum())
			}
		})
	}
}

func TestFakePostgresCosts(t *testing.T) {
	fake := NewFakeClient()
	ctx := context.Background()
	from, to := costDate("2024-02-28"), costDate("2024-03-01")
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
	other, err := fake.DailyForPostgres(ctx, "team", "prod", "orders", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Error("fake cache merged distinct environments")
	}
	beforeData := costDate("2023-01-01")
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
