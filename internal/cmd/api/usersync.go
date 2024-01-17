package api

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/usersync"
	"github.com/sirupsen/logrus"
)

func runUserSync(ctx context.Context, cancel context.CancelFunc, cfg *Config, db database.Database, log logrus.FieldLogger, userSync chan uuid.UUID, userSyncRuns *usersync.RunsHandler) {
	if !cfg.UserSync.Enabled {
		log.Infof("user sync is disabled")
		for sync := range userSync {
			// drain channel
			log.Infof("draining user sync request with correlation ID %s", sync)
		}
		return
	}

	defer cancel()

	userSyncer, err := usersync.NewFromConfig(cfg.GoogleManagementProjectID, cfg.TenantDomain, cfg.UserSync.AdminGroupPrefix, db, log, userSyncRuns)
	if err != nil {
		log.WithError(err).Errorf("unable to set up user syncer")
		return
	}

	userSyncTimer := time.NewTimer(1 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return

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
