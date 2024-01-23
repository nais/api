package fixtures

import (
	"context"

	db "github.com/nais/api/internal/database"
	"github.com/sirupsen/logrus"
)

func SetupDefaultReconcilers(ctx context.Context, log *logrus.Entry, reconcilers []string, database db.Database) error {
	if len(reconcilers) == 0 {
		log.Infof("API_BACKEND_FIRST_RUN_ENABLE_RECONCILERS not set or empty - not enabling any reconcilers")
		return nil
	}

	log.Infof("enablling reconcilers: %v", reconcilers)
	for _, reconciler := range reconcilers {
		_, err := database.EnableReconciler(ctx, reconciler)
		if err != nil {
			return err
		}
	}

	return nil
}
