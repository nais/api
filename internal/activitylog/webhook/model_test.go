package webhook

import (
	"testing"

	"github.com/nais/api/internal/slug"
)

func TestWebhookSubscription_MatchesEvent(t *testing.T) {
	teamA := slug.Slug("team-a")
	teamB := slug.Slug("team-b")

	tests := []struct {
		name  string
		sub   WebhookSubscription
		event WebhookEvent
		want  bool
	}{
		{
			name: "disabled subscription never matches",
			sub: WebhookSubscription{
				Enabled:    false,
				EventTypes: []string{"*"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
		{
			name: "global wildcard subscription matches any event",
			sub: WebhookSubscription{
				Enabled:    true,
				TeamSlug:   nil,
				EventTypes: []string{"*"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  true,
		},
		{
			name: "team-scoped subscription matches same team and type",
			sub: WebhookSubscription{
				Enabled:    true,
				TeamSlug:   &teamA,
				EventTypes: []string{"TEAM_MEMBER_ADDED"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  true,
		},
		{
			name: "team-scoped subscription does not match a different team",
			sub: WebhookSubscription{
				Enabled:    true,
				TeamSlug:   &teamA,
				EventTypes: []string{"*"},
			},
			event: WebhookEvent{TeamSlug: &teamB, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
		{
			name: "team-scoped subscription does not match event with no team",
			sub: WebhookSubscription{
				Enabled:    true,
				TeamSlug:   &teamA,
				EventTypes: []string{"*"},
			},
			event: WebhookEvent{TeamSlug: nil, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
		{
			name: "subscription event types include one of the event's activity types",
			sub: WebhookSubscription{
				Enabled:    true,
				TeamSlug:   &teamA,
				EventTypes: []string{"TEAM_MEMBER_REMOVED", "TEAM_MEMBER_ADDED"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  true,
		},
		{
			name: "subscription event types do not include any of the event's activity types",
			sub: WebhookSubscription{
				Enabled:    true,
				TeamSlug:   &teamA,
				EventTypes: []string{"TEAM_MEMBER_REMOVED"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sub.MatchesEvent(tt.event); got != tt.want {
				t.Errorf("MatchesEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}
