package webhook

import (
	"testing"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
)

func TestProcessDelivery_SkipsNewlyIgnoredQueuedEvent(t *testing.T) {
	const activityType = "WEBHOOK_TEST_QUEUED_IGNORED_EVENT"
	const resourceType = "WEBHOOK_TEST_QUEUED_IGNORED_RESOURCE"
	activitylog.RegisterActivityType(activityType, activitylog.ActivityLogEntryActionCreated, resourceType)
	entry := &webhooksql.ActivityLogEntry{
		ResourceType: resourceType,
		Action:       string(activitylog.ActivityLogEntryActionCreated),
	}
	activitylog.RegisterActivityType(activityType, activitylog.ActivityLogEntryActionCreated, resourceType, activitylog.IgnoreWebhook())

	for _, eventTypes := range [][]string{{"*"}, {activityType}} {
		for _, retryCount := range []int32{0, 1} {
			d := &Dispatcher{}
			// Ignored deliveries must return before using HTTP, database, or metrics dependencies.
			d.processDelivery(t.Context(), nil,
				&webhooksql.WebhookEventDelivery{RetryCount: retryCount},
				&webhooksql.WebhookSubscription{EventTypes: eventTypes, Enabled: true},
				entry,
			)
		}
	}
}
