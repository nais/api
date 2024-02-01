package graph

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"k8s.io/utils/ptr"
)

func toGraphTeams(m []*database.Team) []*model.Team {
	ret := make([]*model.Team, 0)
	for _, team := range m {
		ret = append(ret, loader.ToGraphTeam(team))
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

func toGraphReconcilerResource(r *database.ReconcilerResource) *model.ReconcilerResource {
	var metadata *string
	if len(r.Metadata) > 0 {
		metadata = ptr.To(string(r.Metadata))
	}

	return &model.ReconcilerResource{
		ID:         r.ID,
		Reconciler: r.ReconcilerName,
		Key:        r.Name,
		Value:      r.Value,
		Metadata:   metadata,
		GQLVars: model.ReconcilerResourceGQLVars{
			Team: r.TeamSlug,
		},
	}
}
