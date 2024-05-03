package teamsearch

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
)

type TeamSearcher struct {
	db database.Database
}

func New(db database.Database) *TeamSearcher {
	return &TeamSearcher{
		db: db,
	}
}

func (d *TeamSearcher) SupportsSearchFilter(filter *model.SearchFilter) bool {
	return filter == nil || filter.Type == nil || *filter.Type == model.SearchTypeTeam
}

func (d *TeamSearcher) Search(ctx context.Context, q string, _ *model.SearchFilter) []*search.Result {
	allTeamSlugs, err := d.db.GetAllTeamSlugs(ctx)
	if err != nil {
		return nil
	}

	ranks := make(map[slug.Slug]int)
	matchingSlugs := make([]slug.Slug, 0)
	for _, s := range allTeamSlugs {
		if rank := search.Match(q, string(s)); rank >= 0 {
			ranks[s] = rank
			matchingSlugs = append(matchingSlugs, s)
		}
	}

	teams, err := d.db.GetTeamsBySlugs(ctx, matchingSlugs)
	if err != nil {
		return nil
	}

	ret := make([]*search.Result, len(teams))
	for i, team := range teams {
		ret[i] = &search.Result{
			Rank: ranks[team.Slug],
			Node: &model.Team{
				Slug:               team.Slug,
				Purpose:            team.Purpose,
				LastSuccessfulSync: &team.LastSuccessfulSync.Time,
				SlackChannel:       team.SlackChannel,
			},
		}
	}
	return ret
}
