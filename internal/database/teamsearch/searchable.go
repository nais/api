package teamsearch

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/search"
)

type TeamSearcher struct {
	db database.Database
}

func New(db database.Database) *TeamSearcher {
	return &TeamSearcher{
		db: db,
	}
}

func (d *TeamSearcher) Search(ctx context.Context, q string, filter *model.SearchFilter) []*search.Result {
	if !isTeamFilterOrNoFilter(filter) {
		return nil
	}

	ret, err := d.db.SearchTeams(ctx, q, 10)
	if err != nil {
		return nil
	}

	edges := make([]*search.Result, 0)
	for _, team := range ret {
		rank := search.Match(q, team.Slug.String())
		if rank == -1 {
			continue
		}
		edges = append(edges, &search.Result{
			Rank: rank,
			Node: &model.Team{
				Slug:               team.Slug,
				Purpose:            team.Purpose,
				LastSuccessfulSync: &team.LastSuccessfulSync.Time,
				SlackChannel:       team.SlackChannel,
			},
		})
	}
	return edges
}

// isTeamFilterOrNoFilter returns true if the filter is a team filter or no filter is provided
func isTeamFilterOrNoFilter(filter *model.SearchFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Type == nil {
		return true
	}

	return *filter.Type == model.SearchTypeTeam
}
