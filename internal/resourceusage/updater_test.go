package resourceusage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/resourceusage"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func Test_updater_UpdateResourceUsage(t *testing.T) {
	ctx := context.Background()
	t.Run("error when fetching max timestamp from database", func(t *testing.T) {
		querier := gensql.NewMockQuerier(t)
		querier.EXPECT().MaxResourceUtilizationDate(ctx).Return(pgtype.Timestamptz{}, assert.AnError)
		log, _ := logrustest.NewNullLogger()
		updater := resourceusage.NewUpdater(nil, nil, querier, log)
		rowsUpserted, err := updater.UpdateResourceUsage(ctx)
		assert.Equal(t, 0, rowsUpserted)
		assert.ErrorContains(t, err, "unable to fetch max timestamp from database")
	})
}
