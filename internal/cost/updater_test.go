package cost_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/cost"
	"github.com/nais/api/internal/database/gensql"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/option"
)

const (
	bigQueryHost = "0.0.0.0:9050"
	bigQueryUrl  = "http://" + bigQueryHost
	projectID    = "nais-io"
	tenant       = "test"
	daysToFetch  = 3650
	chanSize     = 1000
	YYYYMMDD     = "2006-01-02"
)

var costUpdaterOpts = []cost.Option{
	cost.WithDaysToFetch(daysToFetch),
}

func TestUpdater_FetchBigQueryData(t *testing.T) {
	_, err := net.DialTimeout("tcp", bigQueryHost, 10*time.Millisecond)
	if err != nil {
		t.Skipf("BigQuery is not available on "+bigQueryHost+" (%s), skipping test. You can start the service with `docker compose up -d`", err)
	}

	ctx := context.Background()
	querier := gensql.NewMockQuerier(t)
	logger, _ := logrustest.NewNullLogger()
	bigQueryClient, err := bigquery.NewClient(
		ctx,
		projectID,
		option.WithEndpoint(bigQueryUrl),
		option.WithoutAuthentication(),
	)
	assert.NoError(t, err)
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
		assert.ErrorContains(t, err, "Table not found")
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

		assert.NoError(t, err)
		assert.Len(t, ch, 100)

		var row gensql.CostUpsertParams
		var ok bool

		row, ok = <-ch
		assert.True(t, ok)
		assert.Equal(t, "dev", *row.Env)
		assert.Equal(t, "team-1-app-1", row.App)
		assert.Equal(t, "team-1", *row.Team)
		assert.Equal(t, "Cloud SQL", row.CostType)
		assert.Equal(t, "2023-08-31", row.Date.Time.Format(YYYYMMDD))
		assert.Equal(t, float32(0.204017), row.DailyCost)

		// jump ahead some results
		for i := 0; i < 42; i++ {
			_, ok = <-ch
			assert.True(t, ok)
		}

		row, ok = <-ch
		assert.True(t, ok)
		assert.Equal(t, "dev", *row.Env)
		assert.Equal(t, "team-2-app-1", row.App)
		assert.Equal(t, "team-2", *row.Team)
		assert.Equal(t, "Cloud SQL", row.CostType)
		assert.Equal(t, "2023-09-01", row.Date.Time.Format(YYYYMMDD))
		assert.Equal(t, float32(0.288296), row.DailyCost)
	})
}

func TestUpdater_ShouldUpdateCosts(t *testing.T) {
	ctx := context.Background()
	logger, _ := logrustest.NewNullLogger()
	bigQueryClient, err := bigquery.NewClient(ctx, projectID, option.WithoutAuthentication())
	assert.NoError(t, err)

	t.Run("error when fetching last date", func(t *testing.T) {
		querier := gensql.NewMockQuerier(t)
		querier.EXPECT().LastCostDate(ctx).Return(date(time.Now()), fmt.Errorf("some error from the database"))

		shouldUpdate, err := cost.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			costUpdaterOpts...,
		).ShouldUpdateCosts(ctx)
		assert.False(t, shouldUpdate)
		assert.EqualError(t, err, "some error from the database")
	})

	t.Run("last date is current day", func(t *testing.T) {
		querier := gensql.NewMockQuerier(t)
		querier.EXPECT().LastCostDate(ctx).Return(date(time.Now()), nil)

		shouldUpdate, err := cost.NewCostUpdater(
			bigQueryClient,
			querier,
			tenant,
			logger,
			costUpdaterOpts...,
		).ShouldUpdateCosts(ctx)
		assert.False(t, shouldUpdate)
		assert.NoError(t, err)
	})
}

func date(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}
