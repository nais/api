package graph

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func toGraphTeam(m *database.Team) *model.Team {
	ret := &model.Team{
		ID:           scalar.TeamIdent(m.Slug),
		Slug:         m.Slug,
		Purpose:      m.Purpose,
		SlackChannel: m.SlackChannel,
	}

	if m.LastSuccessfulSync.Valid {
		ret.LastSuccessfulSync = &m.LastSuccessfulSync.Time
	}

	return ret
}

func toGraphTeams(m []*database.Team) []*model.Team {
	ret := make([]*model.Team, 0)
	for _, team := range m {
		ret = append(ret, toGraphTeam(team))
	}
	return ret
}

func toGraphTeamDeleteKey(m *database.TeamDeleteKey) *model.TeamDeleteKey {
	return &model.TeamDeleteKey{
		Key:       m.Key.String(),
		CreatedAt: m.CreatedAt.Time,
		Expires:   m.Expires(),
	}
}

func (r *Resolver) hasAccess(ctx context.Context, teamName slug.Slug) bool {
	// Replace with RBAC
	return false
}
