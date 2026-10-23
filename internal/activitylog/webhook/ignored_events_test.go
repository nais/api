package webhook_test

import (
	"slices"
	"testing"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/activitylog/webhook"
	_ "github.com/nais/api/internal/kubernetes/event/pubsublog"
	"github.com/nais/api/internal/slug"
	_ "github.com/nais/api/internal/tunnel"
	_ "github.com/nais/api/internal/unleash"
	_ "github.com/nais/api/internal/vulnerability"
	_ "github.com/nais/api/internal/workload/application"
	_ "github.com/nais/api/internal/workload/job"
)

func TestIgnoredEventTypes(t *testing.T) {
	team := slug.Slug("my-team")
	catalogue := activitylog.KnownEventTypes()
	for _, tt := range []struct {
		activityType activitylog.ActivityLogActivityType
		resourceType string
		action       string
	}{
		{"DEPLOYMENT", "APP", "DEPLOYMENT"},
		{"DEPLOYMENT", "JOB", "DEPLOYMENT"},
		{"APPLICATION_SCALED", "APP", "AUTOSCALE"},
		{"CLUSTER_AUDIT", "CLUSTER_AUDIT", "CLUSTER_AUDIT"},
		{"VULNERABILITY_UPDATED", "VULNERABILITY", "UPDATED"},
		{"TUNNEL_CREATED", "TUNNEL", "CREATED"},
		{"TUNNEL_DELETED", "TUNNEL", "DELETED"},
		{"UNLEASH_INSTANCE_CREATED", "UNLEASH", "CREATED"},
		{"UNLEASH_INSTANCE_UPDATED", "UNLEASH", "UPDATED"},
		{"UNLEASH_INSTANCE_DELETED", "UNLEASH", "DELETED"},
	} {
		t.Run(string(tt.activityType)+"/"+tt.resourceType, func(t *testing.T) {
			if !activitylog.IsIgnoredForWebhook(tt.activityType) {
				t.Fatal("event type must be ignored for webhooks")
			}
			if !tt.activityType.IsValid() {
				t.Fatal("event type must remain valid for activity logs")
			}
			if slices.ContainsFunc(catalogue, func(info activitylog.WebhookEventTypeInfo) bool {
				return info.Type == tt.activityType
			}) {
				t.Error("ignored event type is present in the webhook catalogue")
			}

			resolved := activitylog.LookupActivityTypes(tt.resourceType, tt.action)
			if !slices.Contains(resolved, tt.activityType) {
				t.Fatalf("activity log mapping missing: %v", resolved)
			}
			event := webhook.WebhookEvent{TeamSlug: &team}
			for _, at := range resolved {
				event.ActivityTypes = append(event.ActivityTypes, string(at))
			}
			for _, teamSlug := range []*slug.Slug{nil, &team} {
				for _, eventTypes := range [][]string{{"*"}, {string(tt.activityType)}} {
					sub := webhook.WebhookSubscription{
						Enabled:    true,
						TeamSlug:   teamSlug,
						EventTypes: eventTypes,
					}
					if sub.MatchesEvent(event) {
						t.Errorf("ignored event matched subscription: team=%v, types=%v", teamSlug, eventTypes)
					}
				}
			}
		})
	}
}
