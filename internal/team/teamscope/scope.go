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

	teams := slices.Clone(s.TeamSlugs)
	if s.MemberOfViewer == nil || !*s.MemberOfViewer {
		return teams
	}

	memberships := make([]slug.Slug, 0)
	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil || actor.User.IsServiceAccount() {
		return memberships
	}
	for _, role := range actor.Roles {
		if role.TargetTeamSlug == nil {
			continue
		}
		team := *role.TargetTeamSlug
		if (teams == nil || slices.Contains(teams, team)) && !slices.Contains(memberships, team) {
			memberships = append(memberships, team)
		}
	}
	return memberships
}
