package costupdater_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/v1/cost/costsql"

	"cloud.google.com/go/bigquery"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/cost/costupdater"
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

var costUpdaterOpts = []costupdater.Option{
	costupdater.WithDaysToFetch(daysToFetch),
}

func TestUpdater_FetchBigQueryData(t *testing.T) {
	_, err := net.DialTimeout("tcp", bigQueryHost, 100*time.Millisecond)
	if err != nil {
		t.Skipf("BigQuery is not available on "+bigQueryHost+" (%s), skipping test. You can start the service with `docker compose up bigquery -d`", err)
	}

	ctx := context.Background()
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

	t.Run("no teams in database", func(t *testing.T) {
		querier := costsql.NewMockQuerier(t)
		querier.EXPECT().
			ListTeamSlugsForCostUpdater(ctx).
			Return([]slug.Slug{}, nil).
			Once()
		ch := make(chan costsql.CostUpsertParams, chanSize)
		defer close(ch)
		err := costupdater.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			append(costUpdaterOpts, costupdater.WithBigQueryTable("invalid-table"))...,
		).FetchBigQueryData(ctx, ch)
		if contains := "no team slugs"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error to contain %q", contains)
		}
	})

	t.Run("unable to get iterator", func(t *testing.T) {
		querier := costsql.NewMockQuerier(t)
		querier.EXPECT().
			ListTeamSlugsForCostUpdater(ctx).
			Return([]slug.Slug{"team"}, nil).
			Once()
		ch := make(chan costsql.CostUpsertParams, chanSize)
		defer close(ch)
		err := costupdater.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			append(costUpdaterOpts, costupdater.WithBigQueryTable("invalid-table"))...,
		).FetchBigQueryData(ctx, ch)
		if !strings.Contains(err.Error(), "Table not found") {
			t.Error("expected error to contain 'Table not found'")
		}
	})

	t.Run("get data from BigQuery", func(t *testing.T) {
		slugs := []slug.Slug{}
		for i := range 14 {
			slugs = append(slugs, slug.Slug(fmt.Sprintf("team-%d", i+1)))
		}
		querier := costsql.NewMockQuerier(t)
		querier.EXPECT().
			ListTeamSlugsForCostUpdater(ctx).
			Return(slugs, nil).
			Once()

		ch := make(chan costsql.CostUpsertParams, chanSize)
		defer close(ch)

		err := costupdater.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			costUpdaterOpts...,
		).FetchBigQueryData(ctx, ch)
		if err != nil {
			t.Fatal(err)
		}

		if len(ch) != 97 {
			t.Errorf("expected channel to contain 97 items, got %d", len(ch))
		}

		var row costsql.CostUpsertParams
		var ok bool

		row, ok = <-ch
		if !ok {
			t.Fatal("expected channel to contain 100 items")
		}

		want1 := costsql.CostUpsertParams{
			Environment: ptr.To("dev"),
			App:         "team-1-app-1",
			TeamSlug:    slug.Slug("team-1"),
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
		want2 := costsql.CostUpsertParams{
			Environment: ptr.To("dev"),
			App:         "team-2-app-1",
			TeamSlug:    slug.Slug("team-2"),
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
		querier := costsql.NewMockQuerier(t)
		querier.EXPECT().
			LastCostDate(ctx).
			Return(date(time.Now()), fmt.Errorf("some error from the database"))

		shouldUpdate, err := costupdater.NewCostUpdater(
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
		querier := costsql.NewMockQuerier(t)
		querier.EXPECT().
			LastCostDate(ctx).
			Return(date(time.Now()), nil)

		shouldUpdate, err := costupdater.NewCostUpdater(
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
