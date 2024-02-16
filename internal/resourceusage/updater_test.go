package resourceusage_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/resourceusage"
	logrustest "github.com/sirupsen/logrus/hooks/test"
)

func Test_updater_UpdateResourceUsage(t *testing.T) {
	ctx := context.Background()
	t.Run("error when fetching max timestamp from database", func(t *testing.T) {
		querier := database.NewMockDatabase(t)
		querier.EXPECT().
			MaxResourceUtilizationDate(ctx).
			Return(pgtype.Timestamptz{}, fmt.Errorf("some error"))
		log, _ := logrustest.NewNullLogger()
		updater := resourceusage.NewUpdater(nil, nil, querier, log)
		rowsUpserted, err := updater.UpdateResourceUsage(ctx)
		if rowsUpserted != 0 {
			t.Errorf("expected 0 rows upserted, got %v", rowsUpserted)
		}

		if contains := "unable to fetch max timestamp from database"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error to contain %v, got %v", contains, err)
		}
	})
}
