package graph

import (
	"context"

	"github.com/nais/api/internal/auth"
)

func (r *Resolver) hasAccess(ctx context.Context, teamName string) bool {
	email, err := auth.GetEmail(ctx)
	if err != nil {
		r.log.Errorf("getting email from context: %v", err)
		return false
	}

	teams, err := r.teamsClient.GetTeamsForUser(ctx, email)
	if err != nil {
		r.log.Errorf("getting teams from Teams: %v", err)
		return false
	}

	for _, team := range teams {
		if team.Team.Slug == teamName {
			return true
		}
	}

	return false
}
