package dataloader

import (
	"context"
	"fmt"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/metrics"
	"github.com/nais/api/internal/slug"
)

type TeamReader struct {
	db database.Database
}

const LoaderNameTeams = "teams"

// TODO: Deduplicate this with what's in graph pkg
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

func (r *TeamReader) load(ctx context.Context, keys []string) []*dataloader.Result[*model.Team] {
	// TODO (only fetch teams requested by keys var)
	teams, err := r.db.GetAllTeams(ctx)
	if err != nil {
		panic(err)
	}

	teamBySlug := map[string]*model.Team{}
	for _, u := range teams {
		teamBySlug[u.Slug.String()] = toGraphTeam(u)
	}

	output := make([]*dataloader.Result[*model.Team], len(keys))
	for index, teamKey := range keys {
		team, ok := teamBySlug[teamKey]
		if ok {
			output[index] = &dataloader.Result[*model.Team]{Data: team, Error: nil}
		} else {
			err := fmt.Errorf("team not found %q", teamKey)
			output[index] = &dataloader.Result[*model.Team]{Data: nil, Error: err}
		}
	}

	metrics.IncDataloaderLoads(LoaderNameTeams)
	return output
}

func (r *TeamReader) newCache() dataloader.Cache[string, *model.Team] {
	return dataloader.NewCache[string, *model.Team]()
}

func GetTeam(ctx context.Context, teamSlug *slug.Slug) (*model.Team, error) {
	metrics.IncDataloaderCalls(LoaderNameTeams)
	loaders := For(ctx)
	thunk := loaders.TeamsLoader.Load(ctx, teamSlug.String())
	result, err := thunk()
	if err != nil {
		return nil, err
	}
	return result, nil
}
