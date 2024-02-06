package loader

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

type teamReader struct {
	db database.TeamRepo
}

func (t teamReader) getTeams(ctx context.Context, ids []slug.Slug) ([]*model.Team, []error) {
	getID := func(obj *model.Team) slug.Slug { return obj.Slug }
	return loadModels(ctx, ids, t.db.GetTeamsBySlugs, ToGraphTeam, getID)
}

func GetTeam(ctx context.Context, teamSlug slug.Slug) (*model.Team, error) {
	return For(ctx).TeamLoader.Load(ctx, teamSlug)
}

func ToGraphTeam(m *database.Team) *model.Team {
	ret := &model.Team{
		Slug:                   m.Slug,
		Purpose:                m.Purpose,
		SlackChannel:           m.SlackChannel,
		GitHubTeamSlug:         m.GithubTeamSlug,
		AzureGroupID:           m.AzureGroupID,
		GoogleGroupEmail:       m.GoogleGroupEmail,
		GoogleArtifactRegistry: m.GarRepository,
	}

	if m.LastSuccessfulSync.Valid {
		ret.LastSuccessfulSync = &m.LastSuccessfulSync.Time
	}

	return ret
}
