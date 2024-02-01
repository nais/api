package dataloader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/metrics"
	"github.com/nais/api/internal/slug"
)

type TeamReader struct {
	db database.Database
}

const LoaderNameTeams = "teams"

func ToGraphTeam(m *database.Team) *model.Team {
	ret := &model.Team{
		Slug:             m.Slug,
		Purpose:          m.Purpose,
		SlackChannel:     m.SlackChannel,
		GoogleGroupEmail: m.GoogleGroupEmail,
		GitHubTeamSlug:   m.GithubTeamSlug,
		AzureGroupID:     m.AzureGroupID,
	}

	if m.LastSuccessfulSync.Valid {
		ret.LastSuccessfulSync = &m.LastSuccessfulSync.Time
	}

	return ret
}

func (r *TeamReader) load(ctx context.Context, keys []slug.Slug) []*dataloader.Result[*model.Team] {
	// TODO (only fetch teams requested by keys var)
	limit, offset := 100, 0
	teams := make([]*database.Team, 0)
	for {
		page, _, err := r.db.GetTeams(ctx, database.Page{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			panic(err)
		}
		teams = append(teams, page...)
		if len(page) < limit {
			break
		}
		offset += limit
	}

	teamBySlug := map[slug.Slug]*model.Team{}
	for _, u := range teams {
		teamBySlug[u.Slug] = ToGraphTeam(u)
	}

	output := make([]*dataloader.Result[*model.Team], len(keys))
	for index, teamKey := range keys {
		team, ok := teamBySlug[teamKey]
		if ok {
			output[index] = &dataloader.Result[*model.Team]{Data: team, Error: nil}
		} else {
			output[index] = &dataloader.Result[*model.Team]{Data: nil, Error: apierror.ErrTeamNotExist}
		}
	}

	metrics.IncDataloaderLoads(LoaderNameTeams)
	return output
}

func (r *TeamReader) newCache() dataloader.Cache[slug.Slug, *model.Team] {
	return dataloader.NewCache[slug.Slug, *model.Team]()
}

func GetTeam(ctx context.Context, teamSlug slug.Slug) (*model.Team, error) {
	metrics.IncDataloaderCalls(LoaderNameTeams)
	loaders := For(ctx)
	thunk := loaders.TeamsLoader.Load(ctx, teamSlug)
	result, err := thunk()
	if err != nil {
		return nil, err
	}
	return result, nil
}
