package teamscope

import (
	"context"
	"slices"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
)

type TeamScopeInput struct {
	TeamSlugs      []slug.Slug `json:"teamSlugs,omitempty"`
	MemberOfViewer *bool       `json:"memberOfViewer,omitempty"`
}

// Resolve returns nil for an unrestricted selection and an empty slice for no matching teams.
func (s *TeamScopeInput) Resolve(ctx context.Context) []slug.Slug {
	if s == nil {
		return nil
	}

	var teams []slug.Slug
	if s.TeamSlugs != nil {
		teams = make([]slug.Slug, 0, len(s.TeamSlugs))
		for _, team := range s.TeamSlugs {
			if !slices.Contains(teams, team) {
				teams = append(teams, team)
			}
		}
	}
	if s.MemberOfViewer == nil || !*s.MemberOfViewer {
		return teams
	}

	if teams == nil {
		teams = make([]slug.Slug, 0)
	}
	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil || actor.User.IsServiceAccount() {
		return teams
	}
	for _, role := range actor.Roles {
		if role.TargetTeamSlug == nil {
			continue
		}
		team := *role.TargetTeamSlug
		if !slices.Contains(teams, team) {
			teams = append(teams, team)
		}
	}
	return teams
}
