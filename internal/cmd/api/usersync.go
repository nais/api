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
	userSyncInterval = time.Minute * 15
	userSyncTimeout  = time.Second * 30
)

func runUserSync(ctx context.Context, cfg *Config, db database.Database, log logrus.FieldLogger, userSync chan uuid.UUID, userSyncRuns *usersync.RunsHandler) error {
	if !cfg.UserSync.Enabled {
		log.Infof("user sync is disabled")
		for {
			select {
			case <-ctx.Done():
				return nil
			case cID := <-userSync:
				// drain channel
				log.Infof("draining user sync request with correlation ID %s", cID)
			}
		}
	}

	userSyncer, err := usersync.NewFromConfig(ctx, cfg.UserSync.ServiceAccount, cfg.UserSync.SubjectEmail, cfg.TenantDomain, cfg.UserSync.AdminGroupPrefix, db, log, userSyncRuns)
	if err != nil {
		log.WithError(err).Errorf("unable to set up user syncer")
		return err
	}

	userSyncTimer := time.NewTimer(1 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case correlationID := <-userSync:
			if userSyncer == nil {
				log.Infof("user sync is disabled")
				break
			}

			log.Debug("starting user synchronization...")
			ctx, cancel := context.WithTimeout(ctx, userSyncTimeout)
			err = userSyncer.Sync(ctx, correlationID)
			cancel()

			if err != nil {
				log.WithError(err).Error("sync users")
			}

			log.Debugf("user sync complete")

		case <-userSyncTimer.C:
			nextUserSync := time.Now().Add(userSyncInterval)
			userSyncTimer.Reset(userSyncInterval)
			log.Debugf("scheduled user sync triggered; next run at %s", nextUserSync)

			correlationID := uuid.New()

			userSync <- correlationID
		}
	}
}
