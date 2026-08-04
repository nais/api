package webhook

import (
	"strings"
	"testing"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/slug"
)

func init() {
	activitylog.RegisterActivityType("WEBHOOK_TEST_QUERIES_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_QUERIES_RESOURCE")
	activitylog.RegisterActivityType("WEBHOOK_TEST_QUERIES_GLOBAL_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_QUERIES_RESOURCE", activitylog.GlobalOnly())
}

func TestMaskedSecret(t *testing.T) {
	tests := []struct {
		secret string
		want   string
	}{
		{"", "****"},
		{"a", "****"},
		{"abcd", "****"},
		{"abcde", "****bcde"},
		{"supersecretvalue", "****alue"},
	}

	for _, tt := range tests {
		if got := MaskedSecret(tt.secret); got != tt.want {
			t.Errorf("MaskedSecret(%q) = %q, want %q", tt.secret, got, tt.want)
		}
	}
}

func TestMaskedSecret_NeverLeaksMoreThanLastFourChars(t *testing.T) {
	secret := "supersecretvalue"
	masked := MaskedSecret(secret)

	if strings.Contains(masked, secret[:len(secret)-4]) {
		t.Errorf("masked secret %q leaks part of the original secret %q", masked, secret)
	}
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
