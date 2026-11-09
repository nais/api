package webhook

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/slug"
)

func TestDeliveryBodyText(t *testing.T) {
	for _, tt := range []struct {
		name     string
		body     []byte
		wantText *string
	}{
		{name: "no response"},
		{name: "empty response", body: []byte{}, wantText: new("")},
		{name: "text response", body: []byte("hello"), wantText: new("hello")},
		{name: "binary response", body: []byte{0xff, 0x00}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := []byte("{\n  \"test\": true\n}")
			got := toGraphDelivery(&webhooksql.WebhookDelivery{RequestBody: request, ResponseBody: tt.body})
			if diff := cmp.Diff(tt.wantText, got.ResponseBody); diff != "" {
				t.Errorf("unexpected response text (-want +got):\n%s", diff)
			}
			if got.RequestBody != string(request) {
				t.Error("request text changed during conversion")
			}
		})
	}
}

func TestDeliveryAttemptState(t *testing.T) {
	retryAt := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name string
		row  webhooksql.WebhookDelivery
		want *WebhookDeliveryState
	}{
		{
			name: "successful manual test",
			row:  webhooksql.WebhookDelivery{EventType: "WEBHOOK_TEST", Success: true, AttemptCount: 1},
			want: &WebhookDeliveryState{Status: WebhookDeliveryStatusCompleted, AttemptCount: 1},
		},
		{
			name: "failed manual test",
			row:  webhooksql.WebhookDelivery{EventType: "WEBHOOK_TEST", AttemptCount: 1},
			want: &WebhookDeliveryState{Status: WebhookDeliveryStatusFailed, AttemptCount: 1},
		},
		{
			name: "failed attempt retains its retry snapshot without a queue record",
			row: webhooksql.WebhookDelivery{
				AttemptCount: 2,
				NextRetryAt:  pgtype.Timestamptz{Time: retryAt, Valid: true},
			},
			want: &WebhookDeliveryState{Status: WebhookDeliveryStatusFailed, AttemptCount: 2, NextRetryAt: &retryAt},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := toGraphDelivery(&tt.row)
			if diff := cmp.Diff(tt.want, got.DeliveryState); diff != "" {
				t.Errorf("unexpected attempt state (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateSourceTeams(t *testing.T) {
	owner := slug.Slug("team-owner")
	other := slug.Slug("team-other")
	for _, tt := range []struct {
		name    string
		owner   *slug.Slug
		teams   []slug.Slug
		want    []slug.Slug
		wantErr bool
	}{
		{name: "own team", owner: &owner, teams: []slug.Slug{owner}, want: []slug.Slug{owner}},
		{name: "other teams without owner", owner: &owner, teams: []slug.Slug{other}, want: []slug.Slug{other}},
		{name: "deduplicate", owner: &owner, teams: []slug.Slug{other, owner, other}, want: []slug.Slug{other, owner}},
		{name: "all teams", owner: &owner, teams: []slug.Slug{}, want: []slug.Slug{}},
		{name: "invalid slug", owner: &owner, teams: []slug.Slug{"!"}, wantErr: true},
		{name: "legacy global", want: []slug.Slug{}},
		{name: "global cannot select teams", teams: []slug.Slug{other}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateSourceTeams(tt.owner, tt.teams)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateSourceTeams() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("unexpected source teams (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func init() {
	activitylog.RegisterActivityType("WEBHOOK_TEST_QUERIES_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_QUERIES_RESOURCE", "Test activity.")
	activitylog.RegisterActivityType("WEBHOOK_TEST_QUERIES_GLOBAL_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_QUERIES_RESOURCE", "Test activity.", activitylog.GlobalOnly())
	activitylog.RegisterActivityType("WEBHOOK_TEST_QUERIES_IGNORED_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_QUERIES_IGNORED_RESOURCE", "Test activity.", activitylog.IgnoreWebhook())
}

func TestValidateEventSelection(t *testing.T) {
	team := slug.Slug("my-team")
	for _, tt := range []struct {
		name      string
		selection *WebhookEventSelection
		wantAll   bool
		wantTypes []string
		wantErr   bool
	}{
		{"all events", &WebhookEventSelection{AllEvents: graphql.OmittableOf(new(true))}, true, []string{}, false},
		{"explicit events", &WebhookEventSelection{EventTypes: graphql.OmittableOf([]string{"WEBHOOK_TEST_QUERIES_EVENT"})}, false, []string{"WEBHOOK_TEST_QUERIES_EVENT"}, false},
		{"both fields", &WebhookEventSelection{AllEvents: graphql.OmittableOf(new(true)), EventTypes: graphql.OmittableOf([]string{"WEBHOOK_TEST_QUERIES_EVENT"})}, false, nil, true},
		{"both with empty list", &WebhookEventSelection{AllEvents: graphql.OmittableOf(new(true)), EventTypes: graphql.OmittableOf([]string{})}, false, nil, true},
		{"both with null", &WebhookEventSelection{AllEvents: graphql.OmittableOf(new(true)), EventTypes: graphql.OmittableOf[[]string](nil)}, false, nil, true},
		{"null all events", &WebhookEventSelection{AllEvents: graphql.OmittableOf[*bool](nil)}, false, nil, true},
		{"null list", &WebhookEventSelection{EventTypes: graphql.OmittableOf[[]string](nil)}, false, nil, true},
		{"false", &WebhookEventSelection{AllEvents: graphql.OmittableOf(new(false))}, false, nil, true},
		{"false with list", &WebhookEventSelection{AllEvents: graphql.OmittableOf(new(false)), EventTypes: graphql.OmittableOf([]string{"WEBHOOK_TEST_QUERIES_EVENT"})}, false, nil, true},
		{"missing selection", nil, false, nil, true},
		{"no fields", &WebhookEventSelection{}, false, nil, true},
		{"empty list", &WebhookEventSelection{EventTypes: graphql.OmittableOf([]string{})}, false, nil, true},
		{"wildcard", &WebhookEventSelection{EventTypes: graphql.OmittableOf([]string{"*"})}, false, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, teamSlug := range []*slug.Slug{nil, &team} {
				all, types, err := validateEventSelection(teamSlug, tt.selection)
				if (err != nil) != tt.wantErr {
					t.Fatalf("validateEventSelection() error = %v, wantErr %v", err, tt.wantErr)
				}
				if !tt.wantErr {
					if all != tt.wantAll {
						t.Errorf("allEvents = %v, want %v", all, tt.wantAll)
					}
					if diff := cmp.Diff(tt.wantTypes, types); diff != "" {
						t.Errorf("eventTypes mismatch (-want +got):\n%s", diff)
					}
				}
			}
		})
	}
}

func TestValidateEventTypes_UserFacingErrors(t *testing.T) {
	team := slug.Slug("my-team")
	for _, tt := range []struct {
		name      string
		teamSlug  *slug.Slug
		eventType string
	}{
		{"unknown", nil, "WEBHOOK_TEST_QUERIES_UNKNOWN"},
		{"excluded", nil, "WEBHOOK_TEST_QUERIES_IGNORED_EVENT"},
		{"global-only", &team, "WEBHOOK_TEST_QUERIES_GLOBAL_EVENT"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEventTypes(tt.teamSlug, []string{tt.eventType})
			var apiErr apierror.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected a user-facing API error, got %v", err)
			}
		})
	}
}

func TestValidateEventTypes_IgnoredActivityType(t *testing.T) {
	team := slug.Slug("my-team")
	for _, teamSlug := range []*slug.Slug{nil, &team} {
		for _, eventTypes := range [][]string{
			{"WEBHOOK_TEST_QUERIES_IGNORED_EVENT"},
			{"WEBHOOK_TEST_QUERIES_EVENT", "WEBHOOK_TEST_QUERIES_IGNORED_EVENT"},
		} {
			err := validateEventTypes(teamSlug, eventTypes)
			if err == nil || !strings.Contains(err.Error(), "excluded from webhooks") {
				t.Errorf("validateEventTypes(%v, %v) = %v, want ignored event error", teamSlug, eventTypes, err)
			}
		}
	}
}

func TestValidateEventTypes_TeamScopedWebhookCannotSubscribeToGlobalOnlyType(t *testing.T) {
	team := slug.Slug("my-team")

	if err := validateEventTypes(&team, []string{"WEBHOOK_TEST_QUERIES_GLOBAL_EVENT"}); err == nil {
		t.Error("expected an error when a team-scoped webhook subscribes to a global-only event type")
	}
}

func TestValidateEventTypes_TeamScopedWebhookCanSubscribeToTeamScopedType(t *testing.T) {
	team := slug.Slug("my-team")

	if err := validateEventTypes(&team, []string{"WEBHOOK_TEST_QUERIES_EVENT"}); err != nil {
		t.Errorf("expected no error for a team-scoped event type, got: %v", err)
	}
}

func TestValidateEventTypes_GlobalWebhookCanSubscribeToAnyValidType(t *testing.T) {
	if err := validateEventTypes(nil, []string{"WEBHOOK_TEST_QUERIES_EVENT"}); err != nil {
		t.Errorf("expected no error for a team-scoped event type on a global webhook, got: %v", err)
	}
	if err := validateEventTypes(nil, []string{"WEBHOOK_TEST_QUERIES_GLOBAL_EVENT"}); err != nil {
		t.Errorf("expected no error for a global-only event type on a global webhook, got: %v", err)
	}
}

func TestWebhookUpdatedFields(t *testing.T) {
	before := &WebhookSubscription{
		Name: "Original webhook",
		URL:  "https://old.example.test/token", Secret: "old-secret",
		EventTypes: []string{"TEAM_UPDATED"}, SourceTeamSlugs: []slug.Slug{"team-owner"}, Enabled: true,
	}
	after := &WebhookSubscription{
		Name: "Renamed webhook",
		URL:  "https://new.example.test/other-token", Secret: "new-secret",
		AllEvents: true, EventTypes: []string{}, SourceTeamSlugs: []slug.Slug{}, Enabled: false,
	}
	want := []*WebhookUpdatedActivityLogEntryDataUpdatedField{
		{Field: "name", OldValue: new("Original webhook"), NewValue: new("Renamed webhook")},
		{Field: "url"},
		{Field: "secret"},
		{Field: "eventTypes", OldValue: new("[TEAM_UPDATED]"), NewValue: new("[]")},
		{Field: "allEvents", OldValue: new("false"), NewValue: new("true")},
		{Field: "sourceTeamSlugs", OldValue: new("[team-owner]"), NewValue: new("[]")},
		{Field: "enabled", OldValue: new("true"), NewValue: new("false")},
	}
	if diff := cmp.Diff(want, webhookUpdatedFields(before, after)); diff != "" {
		t.Errorf("unexpected updated fields (-want +got):\n%s", diff)
	}
	if got := webhookUpdatedFields(before, before); len(got) != 0 {
		t.Errorf("unchanged webhook produced updated fields: %v", got)
	}
}
