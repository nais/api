package graph

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/model"
)

func toGraphTeam(m *database.Team) *model.Team {
	ret := &model.Team{
		Slug:             m.Slug,
		Purpose:          m.Purpose,
		SlackChannel:     m.SlackChannel,
		GoogleGroupEmail: m.GoogleGroupEmail,
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

func toGraphTeamMemberReconcilers(tmoors []*gensql.GetTeamMemberOptOutsRow) []*model.TeamMemberReconciler {
	ret := make([]*model.TeamMemberReconciler, 0)
	for _, tmoor := range tmoors {
		ret = append(ret, &model.TeamMemberReconciler{
			Enabled: tmoor.Enabled,
		})
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
