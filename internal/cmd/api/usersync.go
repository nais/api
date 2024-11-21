package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/leaderelection"
	"github.com/nais/api/internal/usersync"
	"github.com/sirupsen/logrus"
)

const (
	usersyncInterval = time.Minute * 15
	usersyncTimeout  = time.Minute
)

func runUsersync(ctx context.Context, pool *pgxpool.Pool, cfg *Config, log logrus.FieldLogger) error {
	if !cfg.Usersync.Enabled {
		log.Warningf("usersync is not enabled")
		return nil
	}

	usersyncer, err := usersync.NewFromConfig(ctx, pool, cfg.Usersync.ServiceAccount, cfg.Usersync.SubjectEmail, cfg.TenantDomain, cfg.Usersync.AdminGroupPrefix, log)
	if err != nil {
		log.WithError(err).Errorf("unable to set up usersyncer")
		return err
	}

	for {
		func() {
			if !leaderelection.IsLeader() {
				log.Debug("not leader, skipping usersync")
				return
			}

			correlationID := uuid.New()
			log := log.WithField("correlation_id", correlationID)
			log.Debugf("starting usersync...")

			ctx, cancel := context.WithTimeout(ctx, usersyncTimeout)
			defer cancel()

			start := time.Now()
			if err := usersyncer.Sync(ctx, correlationID); err != nil {
				log.WithError(err).Errorf("sync users")
			}

			if err := usersyncer.RegisterRun(ctx, correlationID, start, time.Now(), err); err != nil {
				log.WithError(err).Errorf("create usersync run")
			}

			log.WithField("duration", time.Since(start)).Infof("usersync complete")
		}()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(usersyncInterval):
		}
	}
}
