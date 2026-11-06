package webhook

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/slug"
)

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
	activitylog.RegisterActivityType("WEBHOOK_TEST_QUERIES_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_QUERIES_RESOURCE")
	activitylog.RegisterActivityType("WEBHOOK_TEST_QUERIES_GLOBAL_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_QUERIES_RESOURCE", activitylog.GlobalOnly())
	activitylog.RegisterActivityType("WEBHOOK_TEST_QUERIES_IGNORED_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_QUERIES_IGNORED_RESOURCE", activitylog.IgnoreWebhook())
}

func TestValidateEventTypes_Wildcard(t *testing.T) {
	team := slug.Slug("my-team")

	if err := validateEventTypes(&team, []string{"*"}); err != nil {
		t.Errorf("expected wildcard to always be valid for team-scoped webhooks, got error: %v", err)
	}
	if err := validateEventTypes(nil, []string{"*"}); err != nil {
		t.Errorf("expected wildcard to always be valid for global webhooks, got error: %v", err)
	}
}

func TestValidateEventTypes_UnknownActivityType(t *testing.T) {
	if err := validateEventTypes(nil, []string{"WEBHOOK_TEST_QUERIES_UNKNOWN"}); err == nil {
		t.Error("expected an error for an unknown activity type")
	}
}

func TestValidateEventTypes_IgnoredActivityType(t *testing.T) {
	team := slug.Slug("my-team")
	for _, teamSlug := range []*slug.Slug{nil, &team} {
		for _, eventTypes := range [][]string{
			{"WEBHOOK_TEST_QUERIES_IGNORED_EVENT"},
			{"*", "WEBHOOK_TEST_QUERIES_IGNORED_EVENT"},
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
		URL: "https://old.example.test/token", Secret: "old-secret",
		EventTypes: []string{"TEAM_UPDATED"}, SourceTeamSlugs: []slug.Slug{"team-owner"}, Enabled: true,
	}
	after := &WebhookSubscription{
		URL: "https://new.example.test/other-token", Secret: "new-secret",
		EventTypes: []string{"TEAM_MEMBER_ADDED"}, SourceTeamSlugs: []slug.Slug{}, Enabled: false,
	}
	want := []*WebhookUpdatedActivityLogEntryDataUpdatedField{
		{Field: "url"},
		{Field: "secret"},
		{Field: "eventTypes", OldValue: new("[TEAM_UPDATED]"), NewValue: new("[TEAM_MEMBER_ADDED]")},
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
