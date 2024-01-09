package dataloader

import (
	"context"
	"fmt"

	"github.com/graph-gophers/dataloader/v7"
	db "github.com/nais/api/internal/database"
	"github.com/nais/api/internal/metrics"
	"github.com/nais/api/internal/slug"
)

type TeamReader struct {
	db db.Database
}

const LoaderNameTeams = "teams"

func (r *TeamReader) load(ctx context.Context, keys []string) []*dataloader.Result[*db.Team] {
	// TODO (only fetch teams requested by keys var)
	teams, err := r.db.GetAllTeams(ctx)
	if err != nil {
		panic(err)
	}

	teamBySlug := map[string]*db.Team{}
	for _, u := range teams {
		teamBySlug[u.Slug.String()] = u
	}

	output := make([]*dataloader.Result[*db.Team], len(keys))
	for index, teamKey := range keys {
		team, ok := teamBySlug[teamKey]
		if ok {
			output[index] = &dataloader.Result[*db.Team]{Data: team, Error: nil}
		} else {
			err := fmt.Errorf("team not found %q", teamKey)
			output[index] = &dataloader.Result[*db.Team]{Data: nil, Error: err}
		}
	}

	metrics.IncDataloaderLoads(LoaderNameTeams)
	return output
}

func (r *TeamReader) newCache() dataloader.Cache[string, *db.Team] {
	return dataloader.NewCache[string, *db.Team]()
}

func GetTeam(ctx context.Context, teamSlug *slug.Slug) (*db.Team, error) {
	metrics.IncDataloaderCalls(LoaderNameTeams)
	loaders := For(ctx)
	thunk := loaders.TeamsLoader.Load(ctx, teamSlug.String())
	result, err := thunk()
	if err != nil {
		return nil, err
	}
	return result, nil
}
