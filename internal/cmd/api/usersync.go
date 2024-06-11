package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/usersync"
	"github.com/sirupsen/logrus"
)

const (
	usersyncInterval = time.Minute * 15
	usersyncTimeout  = time.Minute
)

func runUsersync(ctx context.Context, cfg *Config, db database.Database, log logrus.FieldLogger, usersyncTrigger chan uuid.UUID) error {
	if !cfg.Usersync.Enabled {
		log.Infof("usersync is not enabled")
		for {
			select {
			case <-ctx.Done():
				return nil
			case correlationID := <-usersyncTrigger:
				log.WithField("correlation_id", correlationID).Infof("usersync is not enabled, draining request")
			}
		}
	}

	usersyncer, err := usersync.NewFromConfig(ctx, cfg.Usersync.ServiceAccount, cfg.Usersync.SubjectEmail, cfg.TenantDomain, cfg.Usersync.AdminGroupPrefix, db, log)
	if err != nil {
		log.WithError(err).Errorf("unable to set up usersyncer")
		return err
	}

	usersyncTrigger <- uuid.New()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case correlationID := <-usersyncTrigger:
			func() {
				log := log.WithField("correlation_id", correlationID)
				log.Debugf("starting usersync...")

				ctx, cancel := context.WithTimeout(ctx, usersyncTimeout)
				defer cancel()

				start := time.Now()
				err := usersyncer.Sync(ctx, correlationID)
				if err != nil {
					log.WithError(err).Errorf("sync users")
				}

				if err := db.CreateUsersyncRun(ctx, correlationID, start, time.Now(), err); err != nil {
					log.WithError(err).Errorf("create usersync run")
				}

				log.WithField("duration", time.Since(start)).Infof("usersync complete")
			}()

		case <-time.After(usersyncInterval):
			usersyncTrigger <- uuid.New()
		}
	}
}
