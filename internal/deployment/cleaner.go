package deployment

import (
	"context"
	"time"

	"github.com/nais/api/internal/deployment/deploymentsql"
	"github.com/nais/api/internal/leaderelection"
	"github.com/sirupsen/logrus"
)

const cleanupSchedule = 1 * time.Hour

type cleaner struct {
	db  deploymentsql.Querier
	log logrus.FieldLogger
}

func RunCleaner(ctx context.Context, dbtx deploymentsql.DBTX, log logrus.FieldLogger) {
	c := &cleaner{
		db:  deploymentsql.New(dbtx),
		log: log,
	}

	for {
		if err := c.cleanDeployments(ctx); err != nil {
			log.WithError(err).Error("error cleaning deployments")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cleanupSchedule):
		}
	}
}

func (c *cleaner) cleanDeployments(ctx context.Context) error {
	if !leaderelection.IsLeader() {
		return nil
	}

	res, err := c.db.CleanupNaisVerification(ctx)
	if err != nil {
		return err
	}

	c.log.WithField("rows_deleted", res.RowsAffected()).Debug("cleaned deployments")
	return nil
}
