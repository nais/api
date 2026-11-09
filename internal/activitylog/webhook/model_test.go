package webhook

import (
	"testing"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/slug"
)

func TestTeamWildcardExcludesGlobalOnlyEvents(t *testing.T) {
	const at activitylog.ActivityLogActivityType = "WEBHOOK_TEST_GLOBAL_ONLY"
	activitylog.RegisterActivityType(at, activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_GLOBAL_ONLY_RESOURCE", "Test activity.", activitylog.GlobalOnly())
	team := slug.Slug("team")
	event := WebhookEvent{TeamSlug: &team, ActivityTypes: []string{string(at)}}
	for _, types := range [][]string{{"*"}, {string(at)}} {
		sub := WebhookSubscription{Enabled: true, TeamSlug: &team, EventTypes: types}
		if sub.MatchesEvent(event) {
			t.Error("team subscription matched a global-only event with a team slug")
		}
		sub.TeamSlug = nil
		if !sub.MatchesEvent(event) {
			t.Error("global subscription must still match global-only events")
		}
	}
}

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
				Enabled:         true,
				TeamSlug:        &teamA,
				SourceTeamSlugs: []slug.Slug{teamA},
				EventTypes:      []string{"TEAM_MEMBER_ADDED"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  true,
		},
		{
			name: "team-scoped subscription does not match a different team",
			sub: WebhookSubscription{
				Enabled:         true,
				TeamSlug:        &teamA,
				SourceTeamSlugs: []slug.Slug{teamA},
				EventTypes:      []string{"*"},
			},
			event: WebhookEvent{TeamSlug: &teamB, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
		{
			name: "team-scoped subscription does not match event with no team",
			sub: WebhookSubscription{
				Enabled:         true,
				TeamSlug:        &teamA,
				SourceTeamSlugs: []slug.Slug{teamA},
				EventTypes:      []string{"*"},
			},
			event: WebhookEvent{TeamSlug: nil, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
		{
			name: "subscription event types include one of the event's activity types",
			sub: WebhookSubscription{
				Enabled:         true,
				TeamSlug:        &teamA,
				SourceTeamSlugs: []slug.Slug{teamA},
				EventTypes:      []string{"TEAM_MEMBER_REMOVED", "TEAM_MEMBER_ADDED"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  true,
		},
		{
			name: "subscription event types do not include any of the event's activity types",
			sub: WebhookSubscription{
				Enabled:         true,
				TeamSlug:        &teamA,
				SourceTeamSlugs: []slug.Slug{teamA},
				EventTypes:      []string{"TEAM_MEMBER_REMOVED"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
		{
			name: "owner can subscribe to another team without subscribing to itself",
			sub: WebhookSubscription{
				Enabled: true, TeamSlug: &teamA, SourceTeamSlugs: []slug.Slug{teamB}, EventTypes: []string{"*"},
			},
			event: WebhookEvent{TeamSlug: &teamB, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  true,
		},
		{
			name: "owner is not implicitly included in explicit source teams",
			sub: WebhookSubscription{
				Enabled: true, TeamSlug: &teamA, SourceTeamSlugs: []slug.Slug{teamB}, EventTypes: []string{"*"},
			},
			event: WebhookEvent{TeamSlug: &teamA, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
		{
			name: "multiple source teams match",
			sub: WebhookSubscription{
				Enabled: true, TeamSlug: &teamA, SourceTeamSlugs: []slug.Slug{teamA, teamB}, EventTypes: []string{"*"},
			},
			event: WebhookEvent{TeamSlug: &teamB, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  true,
		},
		{
			name: "all teams matches another team",
			sub: WebhookSubscription{
				Enabled: true, TeamSlug: &teamA, SourceTeamSlugs: []slug.Slug{}, EventTypes: []string{"*"},
			},
			event: WebhookEvent{TeamSlug: &teamB, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  true,
		},
		{
			name: "all teams excludes global events",
			sub: WebhookSubscription{
				Enabled: true, TeamSlug: &teamA, SourceTeamSlugs: []slug.Slug{}, EventTypes: []string{"*"},
			},
			event: WebhookEvent{ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
			want:  false,
		},
		{
			name: "all teams still filters event types",
			sub: WebhookSubscription{
				Enabled: true, TeamSlug: &teamA, SourceTeamSlugs: []slug.Slug{}, EventTypes: []string{"TEAM_MEMBER_REMOVED"},
			},
			event: WebhookEvent{TeamSlug: &teamB, ActivityTypes: []string{"TEAM_MEMBER_ADDED"}},
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
