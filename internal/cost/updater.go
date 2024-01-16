package cost

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	"k8s.io/utils/ptr"
)

const (
	UpsertBatchSize = 100000
	daysToFetch     = 5
)

// bigQueryCostTableRow is a struct that represents a row in the BigQuery table
type bigQueryCostTableRow struct {
	Env      bigquery.NullString `bigquery:"env"`
	Team     bigquery.NullString `bigquery:"team"`
	App      bigquery.NullString `bigquery:"app"`
	CostType string              `bigquery:"cost_type"`
	Date     civil.Date          `bigquery:"date"`
	Cost     float32             `bigquery:"cost"`
}

// Updater is the cost updater struct
type Updater struct {
	log             logrus.FieldLogger
	db              database.Database
	bigQueryClient  *bigquery.Client
	bigQueryTable   string
	daysToFetch     int
	upsertBatchSize int
}

// Option is a function that can be used to set custom options for the cost updater
type Option func(*Updater)

// WithBigQueryTable will set a custom BigQuery table to fetch data from
func WithBigQueryTable(table string) Option {
	return func(u *Updater) {
		u.bigQueryTable = table
	}
}

// WithDaysToFetch will set a custom number of days to fetch from BigQuery
func WithDaysToFetch(daysToFetch int) Option {
	return func(u *Updater) {
		u.daysToFetch = daysToFetch
	}
}

// NewCostUpdater creates a new cost updater
func NewCostUpdater(bigQueryClient *bigquery.Client, db database.Database, tenantName string, log logrus.FieldLogger, opts ...Option) *Updater {
	updater := &Updater{
		db:              db,
		bigQueryClient:  bigQueryClient,
		log:             log,
		bigQueryTable:   "nais-io.console.cost_" + tenantName,
		daysToFetch:     daysToFetch,
		upsertBatchSize: UpsertBatchSize,
	}

	for _, opt := range opts {
		opt(updater)
	}

	return updater
}

// ShouldUpdateCosts returns true if costs should be updated, false otherwise
func (c *Updater) ShouldUpdateCosts(ctx context.Context) (bool, error) {
	lastDate, err := c.db.LastCostDate(ctx)
	if err != nil {
		return false, err
	}

	if lastDate.Time.Format(time.DateOnly) == time.Now().Format(time.DateOnly) {
		// already have todays date in the costs, no need for another update
		return false, nil
	}

	if time.Now().Hour() < 5 {
		// no need for updating costs until after 05:00 ¯\_(ツ)_/¯
		return false, nil
	}

	return true, nil
}

// FetchBigQueryData fetches cost data from BigQuery and sends it to the provided channel
func (c *Updater) FetchBigQueryData(ctx context.Context, ch chan<- gensql.CostUpsertParams) error {
	start := time.Now()
	numRows := 0
	it, err := c.getBigQueryIterator(ctx)
	if err != nil {
		return err
	}

	var row bigQueryCostTableRow
	for {
		if err := it.Next(&row); err != nil {
			if errors.Is(err, iterator.Done) {
				break
			}

			if errors.Is(err, context.Canceled) {
				return err
			}

			continue
		}

		numRows++

		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- gensql.CostUpsertParams{
			Environment: nullToStringPointer(row.Env),
			TeamSlug:    nullToSlugPointer(row.Team),
			App:         row.App.StringVal,
			CostType:    row.CostType,
			Date:        pgtype.Date{Time: row.Date.In(time.UTC), Valid: true},
			DailyCost:   row.Cost,
		}:
			// entry sent to the channel
		}
	}

	c.log.WithFields(logrus.Fields{
		"duration": time.Since(start),
		"num_rows": numRows,
	}).Infof("done fetching data from BigQuery")
	return nil
}

// UpdateCosts will update the cost data in the database based on data from the provided channel
func (c *Updater) UpdateCosts(ctx context.Context, ch <-chan gensql.CostUpsertParams) error {
	var numUpserted, numErrors int
	start := time.Now()

	for {
		batch, err := c.getBatch(ctx, ch)
		if err != nil {
			return err
		}

		if len(batch) == 0 {
			break
		}

		batchUpserts, batchErrors := c.upsertBatch(ctx, batch)
		numUpserted += batchUpserts
		numErrors += batchErrors
	}

	c.log.WithFields(logrus.Fields{
		"duration":   time.Since(start),
		"num_rows":   numUpserted - numErrors,
		"num_errors": numErrors,
	}).Infof("cost data has been updated")
	return nil
}

// upsertBatch will upsert a batch of cost data
func (c *Updater) upsertBatch(ctx context.Context, batch []gensql.CostUpsertParams) (upserted, errors int) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()
	c.db.CostUpsert(ctx, batch).Exec(func(i int, err error) {
		if err != nil {
			errors++
		}
	})

	upserted += len(batch) - errors
	c.log.WithFields(logrus.Fields{
		"duration":   time.Since(start),
		"num_rows":   upserted,
		"num_errors": errors,
	}).Infof("upserted batch")
	return
}

// getBigQueryIterator will return an iterator for the resultset of the cost query
func (c *Updater) getBigQueryIterator(ctx context.Context) (*bigquery.RowIterator, error) {
	sql := fmt.Sprintf(
		"SELECT * FROM `%s` WHERE `date` >= TIMESTAMP_SUB(CURRENT_DATE(), INTERVAL %d DAY)",
		c.bigQueryTable,
		c.daysToFetch,
	)

	c.log.WithField("query", sql).Infof("fetch data from bigquery")
	return c.bigQueryClient.Query(sql).Read(ctx)
}

// getBatch will return a batch of rows from the provided channel
func (c *Updater) getBatch(ctx context.Context, ch <-chan gensql.CostUpsertParams) ([]gensql.CostUpsertParams, error) {
	batch := make([]gensql.CostUpsertParams, 0)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case row, more := <-ch:
			if !more {
				return batch, nil
			}

			batch = append(batch, row)
			if len(batch) == c.upsertBatchSize {
				return batch, nil
			}
		}
	}
}

// nullToStringPointer converts a bigquery.NullString to a *string
func nullToStringPointer(s bigquery.NullString) *string {
	if s.Valid {
		return &s.StringVal
	}
	return nil
}

// nullToStringPointer converts a bigquery.NullString to a *string
func nullToSlugPointer(s bigquery.NullString) *slug.Slug {
	if s.Valid {
		return ptr.To(slug.Slug(s.StringVal))
	}
	return nil
}
