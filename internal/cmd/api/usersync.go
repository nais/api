package api

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/leaderelection"
	"github.com/nais/api/internal/usersync/usersyncer"
	"github.com/sirupsen/logrus"
	"github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/client/middleware"
	zitadeluser "github.com/zitadel/zitadel-go/v3/pkg/client/user/v2"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel"
)

const (
	usersyncInterval = time.Minute * 15
	usersyncTimeout  = time.Minute * 5
)

func runUsersync(ctx context.Context, pool *pgxpool.Pool, cfg *Config, log logrus.FieldLogger) error {
	if !cfg.Usersync.Enabled {
		log.Warningf("usersync is not enabled")
		return nil
	}

	var zw *usersyncer.ZitadelWrapper
	if cfg.Zitadel.Domain != "" && cfg.Zitadel.Key != "" && cfg.Zitadel.IDPID != "" {
		key, err := base64.StdEncoding.DecodeString(cfg.Zitadel.Key)
		if err != nil {
			return err
		}

		zc, err := zitadeluser.NewClient(ctx, "https://"+cfg.Zitadel.Domain, cfg.Zitadel.Domain+":443",
			[]string{oidc.ScopeOpenID, client.ScopeZitadelAPI()},
			zitadel.WithJWTProfileTokenSource(middleware.JWTProfileFromFileData(ctx, key)))
		if err != nil {
			return err
		}

		zw = &usersyncer.ZitadelWrapper{
			Client: zc,
			IDP:    cfg.Zitadel.IDPID,
		}
	}

	us, err := usersyncer.NewFromConfig(ctx, pool, cfg.Usersync.ServiceAccount, cfg.Usersync.SubjectEmail, cfg.TenantDomain, cfg.Usersync.AdminGroupPrefix, zw, log)
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

			log.Debugf("starting usersync...")

			ctx, cancel := context.WithTimeout(ctx, usersyncTimeout)
			defer cancel()

			start := time.Now()
			if err := us.Sync(ctx); err != nil {
				log.WithError(err).Errorf("sync users")
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
