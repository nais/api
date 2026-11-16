package graph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/cost"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/slug"
)

type postgresCostClient struct {
	cost.Client
	team              slug.Slug
	environment, name string
	from, to          time.Time
	calls             int
	result            *cost.PostgresCostPeriod
	err               error
}

func (c *postgresCostClient) DailyForPostgres(_ context.Context, team slug.Slug, environment, name string, from, to time.Time) (*cost.PostgresCostPeriod, error) {
	c.team, c.environment, c.name, c.from, c.to = team, environment, name, from, to
	c.calls++
	return c.result, c.err
}

func (c *postgresCostClient) MonthlyForPostgres(_ context.Context, team slug.Slug, environment, name string) (*cost.PostgresCostPeriod, error) {
	c.team, c.environment, c.name = team, environment, name
	c.calls++
	return c.result, c.err
}

func TestPostgresCostIdentity(t *testing.T) {
	obj := &postgres.Postgres{TeamSlug: "team", EnvironmentName: "prod", Name: "orders"}
	got, err := (&postgresResolver{}).Cost(context.Background(), obj)
	if err != nil {
		t.Fatal(err)
	}
	want := &cost.PostgresCost{TeamSlug: "team", EnvironmentName: "prod", PostgresName: "orders"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("cost identity (-want +got):\n%s", diff)
	}
}

func TestPostgresCostDailyDateValidation(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	obj := &cost.PostgresCost{TeamSlug: "team", EnvironmentName: "dev", PostgresName: "orders"}
	for _, tt := range []struct {
		name      string
		from, to  time.Time
		wantError bool
	}{
		{name: "single day is inclusive", from: today.AddDate(0, 0, -3), to: today.AddDate(0, 0, -3)},
		{name: "today is allowed despite missing billing", from: today, to: today},
		{name: "reversed dates", from: today, to: today.AddDate(0, 0, -1), wantError: true},
		{name: "future date", from: today, to: today.AddDate(0, 0, 1), wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &postgresCostClient{}
			ctx := cost.NewLoaderContext(context.Background(), nil, cost.WithClient(fake))
			got, err := (&postgresCostResolver{}).Daily(ctx, obj, scalar.NewDate(tt.from), scalar.NewDate(tt.to))
			if tt.wantError {
				if err == nil || fake.calls != 0 {
					t.Errorf("invalid dates: result=%v, error=%v, calls=%d", got, err, fake.calls)
				}
				return
			}
			if err != nil || got != nil || fake.calls != 1 {
				t.Fatalf("no samples: result=%v, error=%v, calls=%d", got, err, fake.calls)
			}
			if fake.team != obj.TeamSlug || fake.environment != obj.EnvironmentName || fake.name != obj.PostgresName || !fake.from.Equal(tt.from) || !fake.to.Equal(tt.to) {
				t.Errorf("wrong identity/date scope: %+v", fake)
			}
		})
	}
}

func TestPostgresCostResolversPropagateResultsAndErrors(t *testing.T) {
	queryErr := errors.New("billing unavailable")
	obj := &cost.PostgresCost{TeamSlug: "team", EnvironmentName: "prod", PostgresName: "orders"}
	date := scalar.NewDate(time.Now().UTC().AddDate(0, 0, -3))
	for _, monthly := range []bool{false, true} {
		for _, queryErr := range []error{nil, queryErr} {
			fake := &postgresCostClient{result: &cost.PostgresCostPeriod{}, err: queryErr}
			if queryErr != nil {
				fake.result = nil
			}
			ctx := cost.NewLoaderContext(context.Background(), nil, cost.WithClient(fake))
			var got *cost.PostgresCostPeriod
			var err error
			if monthly {
				got, err = (&postgresCostResolver{}).Monthly(ctx, obj)
			} else {
				got, err = (&postgresCostResolver{}).Daily(ctx, obj, date, date)
			}
			if !errors.Is(err, queryErr) || got != fake.result {
				t.Errorf("monthly=%t: got %v, %v; want %v, %v", monthly, got, err, fake.result, queryErr)
			}
			if fake.team != obj.TeamSlug || fake.environment != obj.EnvironmentName || fake.name != obj.PostgresName || fake.calls != 1 {
				t.Errorf("monthly=%t: wrong Postgres query scope: %+v", monthly, fake)
			}
		}
	}
	fake := &postgresCostClient{}
	ctx := cost.NewLoaderContext(context.Background(), nil, cost.WithClient(fake))
	if got, err := (&postgresCostResolver{}).Monthly(ctx, obj); got != nil || err != nil {
		t.Errorf("missing monthly data = %v, %v; want nil, nil", got, err)
	}
}
