package cost_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/cost"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/api/option"
	"k8s.io/utils/ptr"
)

const (
	bigQueryHost = "0.0.0.0:9050"
	bigQueryUrl  = "http://" + bigQueryHost
	projectID    = "nais-io"
	tenant       = "test"
	daysToFetch  = 3650
	chanSize     = 1000
)

var costUpdaterOpts = []cost.Option{
	cost.WithDaysToFetch(daysToFetch),
}

func TestUpdater_FetchBigQueryData(t *testing.T) {
	_, err := net.DialTimeout("tcp", bigQueryHost, 100*time.Millisecond)
	if err != nil {
		t.Skipf("BigQuery is not available on "+bigQueryHost+" (%s), skipping test. You can start the service with `docker compose up bigquery -d`", err)
	}

	ctx := context.Background()
	querier := database.NewMockDatabase(t)
	logger, _ := logrustest.NewNullLogger()
	bigQueryClient, err := bigquery.NewClient(
		ctx,
		projectID,
		option.WithEndpoint(bigQueryUrl),
		option.WithoutAuthentication(),
	)
	if err != nil {
		t.Fatal(err)
	}
	bigQueryClient.Location = "EU"

	t.Run("unable to get iterator", func(t *testing.T) {
		ch := make(chan gensql.CostUpsertParams, chanSize)
		defer close(ch)
		err := cost.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			append(costUpdaterOpts, cost.WithBigQueryTable("invalid-table"))...,
		).FetchBigQueryData(ctx, ch)
		if !strings.Contains(err.Error(), "Table not found") {
			t.Error("expected error to contain 'Table not found'")
		}
	})

	t.Run("get data from BigQuery", func(t *testing.T) {
		ch := make(chan gensql.CostUpsertParams, chanSize)
		defer close(ch)

		err := cost.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			costUpdaterOpts...,
		).FetchBigQueryData(ctx, ch)
		if err != nil {
			t.Fatal(err)
		}

		if len(ch) != 100 {
			t.Errorf("expected channel to contain 100 items, got %d", len(ch))
		}

		var row gensql.CostUpsertParams
		var ok bool

		row, ok = <-ch
		if !ok {
			t.Fatal("expected channel to contain 100 items")
		}

		want1 := gensql.CostUpsertParams{
			Environment: ptr.To("dev"),
			App:         "team-1-app-1",
			TeamSlug:    ptr.To(slug.Slug("team-1")),
			CostType:    "Cloud SQL",
			Date:        pgtype.Date{Time: time.Date(2023, 8, 31, 0, 0, 0, 0, time.UTC), Valid: true},
			DailyCost:   0.204017,
		}
		if diff := cmp.Diff(want1, row); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}

		// jump ahead some results
		for i := 0; i < 42; i++ {
			_, ok = <-ch
			if !ok {
				t.Fatal("expected channel to contain more items")
			}
		}

		row, ok = <-ch
		if !ok {
			t.Fatal("expected channel to contain 43 items")
		}
		want2 := gensql.CostUpsertParams{
			Environment: ptr.To("dev"),
			App:         "team-2-app-1",
			TeamSlug:    ptr.To(slug.Slug("team-2")),
			CostType:    "Cloud SQL",
			Date:        pgtype.Date{Time: time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			DailyCost:   0.288296,
		}

		if diff := cmp.Diff(want2, row); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}
	})
}

func TestUpdater_ShouldUpdateCosts(t *testing.T) {
	ctx := context.Background()
	logger, _ := logrustest.NewNullLogger()
	bigQueryClient, err := bigquery.NewClient(ctx, projectID, option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("error when fetching last date", func(t *testing.T) {
		querier := database.NewMockDatabase(t)
		querier.EXPECT().LastCostDate(ctx).Return(date(time.Now()), fmt.Errorf("some error from the database"))

		shouldUpdate, err := cost.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			costUpdaterOpts...,
		).ShouldUpdateCosts(ctx)
		if shouldUpdate {
			t.Error("expected shouldUpdate to be false")
		}
		if err.Error() != "some error from the database" {
			t.Errorf("expected error to be 'some error from the database', got %s", err.Error())
		}
	})

	t.Run("last date is current day", func(t *testing.T) {
		querier := database.NewMockDatabase(t)
		querier.EXPECT().LastCostDate(ctx).Return(date(time.Now()), nil)

		shouldUpdate, err := cost.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			costUpdaterOpts...,
		).ShouldUpdateCosts(ctx)
		if shouldUpdate {
			t.Error("expected shouldUpdate to be false")
		}
		if err != nil {
			t.Errorf("expected error to be nil, got %s", err.Error())
		}
	})
}

func date(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}
