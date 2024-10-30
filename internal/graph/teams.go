package graph

import (
	"github.com/nais/api/internal/database"
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
