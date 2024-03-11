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

func toGraphTeamMemberReconcilers(rs []*gensql.GetTeamMemberOptOutsRow) []*model.TeamMemberReconciler {
	rt := make([]*model.TeamMemberReconciler, len(rs))
	for i, r := range rs {
		rt[i] = &model.TeamMemberReconciler{
			Enabled: r.Enabled,
			GQLVars: model.TeamMemberReconcilerGQLVars{
				Name: r.Name,
			},
		}
	}
	return rt
}

func toGraphTeamDeleteKey(m *database.TeamDeleteKey) *model.TeamDeleteKey {
	return &model.TeamDeleteKey{
		Key:       m.Key.String(),
		CreatedAt: m.CreatedAt.Time,
		Expires:   m.Expires(),
		GQLVars: model.TeamDeleteKeyGQLVars{
			TeamSlug: m.TeamSlug,
			UserID:   m.CreatedBy,
		},
	}
}
