package fixtures

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/sirupsen/logrus"
)

func SetupDefaultReconcilers(ctx context.Context, log logrus.FieldLogger, reconcilers []string, db database.ReconcilerRepo) error {
	if len(reconcilers) == 0 {
		log.Infof("API_BACKEND_FIRST_RUN_ENABLE_RECONCILERS not set or empty - not enabling any reconcilers")
		return nil
	}

	log.Infof("enablling reconcilers: %v", reconcilers)
	for _, reconciler := range reconcilers {
		_, err := db.EnableReconciler(ctx, reconciler)
		if err != nil {
			return err
		}
	}

	return nil
}
