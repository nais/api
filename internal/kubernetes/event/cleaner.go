package event

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/kubernetes/event/eventsql"
	"github.com/nais/api/internal/leaderelection"
	"github.com/sirupsen/logrus"
)

const (
	cleanupSchedule = 1 * time.Hour
	retention       = 30 * 24 * time.Hour
)

// RunCleaner periodically deletes workload events older than the retention period.
func RunCleaner(ctx context.Context, dbtx eventsql.DBTX, log logrus.FieldLogger) {
	db := eventsql.New(dbtx)
	for {
		if leaderelection.IsLeader() {
			res, err := db.DeleteOlderThan(ctx, pgtype.Timestamptz{Time: time.Now().Add(-retention), Valid: true})
			if err != nil {
				log.WithError(err).Error("error cleaning workload events")
			} else {
				log.WithField("rows_deleted", res.RowsAffected()).Debug("cleaned workload events")
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cleanupSchedule):
		}
	}
}
