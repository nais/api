package graph

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
)

func toGraphTeams(m []*database.Team) []*model.Team {
	ret := make([]*model.Team, len(m))
	for i, team := range m {
		ret[i] = loader.ToGraphTeam(team)
	}
	return ret
}

func toGraphTeamMemberReconcilers(tmoors []*gensql.GetTeamMemberOptOutsRow) []*model.TeamMemberReconciler {
	ret := make([]*model.TeamMemberReconciler, len(tmoors))
	for i, tmoor := range tmoors {
		ret[i] = &model.TeamMemberReconciler{
			Enabled: tmoor.Enabled,
		}
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
