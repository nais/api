package activitylog

import (
	"context"
	"time"

	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/nais/api/internal/leaderelection"
	"github.com/sirupsen/logrus"
)

const refreshSchedule = 2 * time.Minute

type cleaner struct {
	db  activitylogsql.Querier
	log logrus.FieldLogger
}

func RunRefresher(ctx context.Context, dbtx activitylogsql.DBTX, log logrus.FieldLogger) {
	c := &cleaner{
		db:  activitylogsql.New(dbtx),
		log: log,
	}

	for {
		if err := c.refreshView(ctx); err != nil {
			log.WithError(err).Error("error refreshing activity log view")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(refreshSchedule):
		}
	}
}

func (c *cleaner) refreshView(ctx context.Context) error {
	if !leaderelection.IsLeader() {
		return nil
	}

	return c.db.RefreshMaterializedView(ctx)
}
