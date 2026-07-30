package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	"github.com/nais/api/internal/leaderelection"
	"github.com/sirupsen/logrus"
)

const (
	cleanupInterval = 24 * time.Hour

	// queueRetention controls how long completed/failed rows are kept in the internal
	// processing tables (webhook_events, webhook_event_deliveries) after they've reached
	// a terminal state.
	queueRetention = 7 * 24 * time.Hour

	// deliveryRetention controls how long delivery attempts are kept in the user-facing
	// audit log (webhook_deliveries).
	deliveryRetention = 30 * 24 * time.Hour
)

// RunCleaner periodically prunes old webhook processing and delivery data. Blocks until
// ctx is cancelled. Only the current leader replica performs deletes; other replicas are
// a no-op on each tick.
func RunCleaner(ctx context.Context, dbtx webhooksql.DBTX, log logrus.FieldLogger) {
	q := webhooksql.New(dbtx)

	for {
		if err := clean(ctx, q); err != nil {
			log.WithError(err).Error("cleaning webhook data")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(cleanupInterval):
		}
	}
}

func clean(ctx context.Context, q *webhooksql.Queries) error {
	if !leaderelection.IsLeader() {
		return nil
	}

	now := time.Now()
	queueBefore := pgtype.Timestamptz{Time: now.Add(-queueRetention), Valid: true}
	deliveryBefore := pgtype.Timestamptz{Time: now.Add(-deliveryRetention), Valid: true}

	if err := q.PruneOldOutboxEvents(ctx, queueBefore); err != nil {
		return fmt.Errorf("pruning outbox events: %w", err)
	}

	if err := q.PruneOldEventDeliveries(ctx, queueBefore); err != nil {
		return fmt.Errorf("pruning event deliveries: %w", err)
	}

	if err := q.PruneDeliveries(ctx, deliveryBefore); err != nil {
		return fmt.Errorf("pruning delivery audit log: %w", err)
	}

	return nil
}
