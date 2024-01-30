package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

const teamDeleteKeyLifetime = time.Hour * 1

type TeamRepo interface {
	ConfirmTeamDeleteKey(ctx context.Context, key uuid.UUID) error
	CreateTeam(ctx context.Context, slug slug.Slug, purpose, slackChannel string) (*Team, error)
	CreateTeamDeleteKey(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*TeamDeleteKey, error)
	DeleteTeam(ctx context.Context, teamSlug slug.Slug) error
	GetActiveTeamBySlug(ctx context.Context, slug slug.Slug) (*Team, error)
	GetActiveTeams(ctx context.Context) ([]*Team, error)
	GetAllTeamMembers(ctx context.Context, teamSlug slug.Slug) ([]*User, error)
	GetSlackAlertsChannels(ctx context.Context, teamSlug slug.Slug) (map[string]string, error)
	GetTeamBySlug(ctx context.Context, slug slug.Slug) (*Team, error)
	GetTeamDeleteKey(ctx context.Context, key uuid.UUID) (*TeamDeleteKey, error)
	GetTeamEnvironments(ctx context.Context, teamSlug slug.Slug, offset, limit int) ([]*TeamEnvironment, int, error)
	GetTeamMember(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*User, error)
	GetTeamMemberOptOuts(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) ([]*gensql.GetTeamMemberOptOutsRow, error)
	GetTeamMembers(ctx context.Context, teamSlug slug.Slug, offset, limit int) ([]*User, int, error)
	GetTeamMembersForReconciler(ctx context.Context, teamSlug slug.Slug, reconcilerName string) ([]*User, error)
	GetTeams(ctx context.Context, offset, limit int) ([]*Team, int, error)
	GetTeamsWithPermissionInGitHubRepo(ctx context.Context, repoName, permission string, offset, limit int) ([]*Team, int, error)
	GetUserTeams(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*UserTeam, int, error)
	RemoveSlackAlertsChannel(ctx context.Context, teamSlug slug.Slug, environment string) error
	RemoveUserFromTeam(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error
	SearchTeams(ctx context.Context, slugMatch string, limit int32) ([]*gensql.Team, error)
	SetGoogleGroupEmailForTeam(ctx context.Context, teamSlug slug.Slug, email string) error
	SetLastSuccessfulSyncForTeam(ctx context.Context, teamSlug slug.Slug) error
	SetSlackAlertsChannel(ctx context.Context, teamSlug slug.Slug, environment, channelName string) error
	TeamExists(ctx context.Context, team slug.Slug) (bool, error)
	UpdateTeam(ctx context.Context, teamSlug slug.Slug, purpose, slackChannel *string) (*Team, error)
}

type TeamEnvironment struct {
	*gensql.TeamEnvironment
}

type TeamDeleteKey struct {
	*gensql.TeamDeleteKey
}

func (k TeamDeleteKey) Expires() time.Time {
	return k.CreatedAt.Time.Add(teamDeleteKeyLifetime)
}

func (k TeamDeleteKey) HasExpired() bool {
	return time.Now().After(k.Expires())
}

type Team struct {
	*gensql.Team
}

func (d *database) RemoveUserFromTeam(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error {
	return d.querier.RemoveUserFromTeam(ctx, userID, &teamSlug)
}

func (d *database) UpdateTeam(ctx context.Context, teamSlug slug.Slug, purpose, slackChannel *string) (*Team, error) {
	team, err := d.querier.UpdateTeam(ctx, purpose, slackChannel, teamSlug)
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) CreateTeam(ctx context.Context, slug slug.Slug, purpose, slackChannel string) (*Team, error) {
	team, err := d.querier.CreateTeam(ctx, slug, purpose, slackChannel)
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) GetActiveTeamBySlug(ctx context.Context, slug slug.Slug) (*Team, error) {
	team, err := d.querier.GetActiveTeamBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) GetTeamBySlug(ctx context.Context, slug slug.Slug) (*Team, error) {
	team, err := d.querier.GetTeamBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) GetTeams(ctx context.Context, offset, limit int) ([]*Team, int, error) {
	var teams []*gensql.Team
	var err error
	teams, err = d.querier.GetTeams(ctx, int32(offset), int32(limit))
	if err != nil {
		return nil, 0, err
	}

	collection := make([]*Team, 0)
	for _, team := range teams {
		collection = append(collection, &Team{Team: team})
	}

	total, err := d.querier.GetTeamsCount(ctx)
	if err != nil {
		return nil, 0, err
	}

	return collection, int(total), nil
}

func (d *database) GetTeamEnvironments(ctx context.Context, teamSlug slug.Slug, offset, limit int) ([]*TeamEnvironment, int, error) {
	rows, err := d.querier.GetTeamEnvironments(ctx, teamSlug, int32(offset), int32(limit))
	if err != nil {
		return nil, 0, err
	}

	envs := make([]*TeamEnvironment, len(rows))
	for i, row := range rows {
		envs[i] = &TeamEnvironment{TeamEnvironment: row}
	}

	total, err := d.querier.GetTeamEnvironmentsCount(ctx, teamSlug)
	if err != nil {
		return nil, 0, err
	}

	return envs, int(total), nil
}

func (d *database) GetActiveTeams(ctx context.Context) ([]*Team, error) {
	teams, err := d.querier.GetActiveTeams(ctx)
	if err != nil {
		return nil, err
	}

	collection := make([]*Team, 0)
	for _, team := range teams {
		collection = append(collection, &Team{Team: team})
	}

	return collection, nil
}

func (d *database) GetUserTeams(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*UserTeam, int, error) {
	rows, err := d.querier.GetUserTeams(ctx, userID, int32(offset), int32(limit))
	if err != nil {
		return nil, 0, err
	}

	totalCount, err := d.querier.GetUserTeamsCount(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	teams := make([]*UserTeam, 0)
	for _, row := range rows {
		teams = append(teams, &UserTeam{Team: &row.Team, RoleName: row.RoleName})
	}

	return teams, int(totalCount), nil
}

func (d *database) GetAllTeamMembers(ctx context.Context, teamSlug slug.Slug) ([]*User, error) {
	var rows []*gensql.User
	var err error
	rows, err = d.querier.GetAllTeamMembers(ctx, &teamSlug)
	if err != nil {
		return nil, err
	}

	members := make([]*User, 0)
	for _, row := range rows {
		members = append(members, &User{User: row})
	}

	return members, nil
}

func (d *database) GetTeamMembers(ctx context.Context, teamSlug slug.Slug, offset, limit int) ([]*User, int, error) {
	var rows []*gensql.User
	var err error
	rows, err = d.querier.GetTeamMembers(ctx, &teamSlug, int32(offset), int32(limit))
	if err != nil {
		return nil, 0, err
	}

	members := make([]*User, 0)
	for _, row := range rows {
		members = append(members, &User{User: row})
	}
	total, err := d.querier.GetTeamMembersCount(ctx, &teamSlug)
	if err != nil {
		return nil, 0, err
	}

	return members, int(total), nil
}

func (d *database) GetTeamMember(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*User, error) {
	user, err := d.querier.GetTeamMember(ctx, &teamSlug, userID)
	if err != nil {
		return nil, err
	}

	return &User{User: user}, nil
}

func (d *database) GetTeamMembersForReconciler(ctx context.Context, teamSlug slug.Slug, reconcilerName string) ([]*User, error) {
	rows, err := d.querier.GetTeamMembersForReconciler(ctx, &teamSlug, reconcilerName)
	if err != nil {
		return nil, err
	}

	members := make([]*User, 0)
	for _, row := range rows {
		members = append(members, &User{User: row})
	}

	return members, nil
}

func (d *database) SetLastSuccessfulSyncForTeam(ctx context.Context, teamSlug slug.Slug) error {
	return d.querier.SetLastSuccessfulSyncForTeam(ctx, teamSlug)
}

func (d *database) GetSlackAlertsChannels(ctx context.Context, teamSlug slug.Slug) (map[string]string, error) {
	channels := make(map[string]string)
	rows, err := d.querier.GetSlackAlertsChannels(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		channels[row.Environment] = row.ChannelName
	}

	return channels, nil
}

func (d *database) SetSlackAlertsChannel(ctx context.Context, teamSlug slug.Slug, environment, channelName string) error {
	return d.querier.SetSlackAlertsChannel(ctx, teamSlug, environment, channelName)
}

func (d *database) RemoveSlackAlertsChannel(ctx context.Context, teamSlug slug.Slug, environment string) error {
	return d.querier.RemoveSlackAlertsChannel(ctx, teamSlug, environment)
}

func (d *database) CreateTeamDeleteKey(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*TeamDeleteKey, error) {
	deleteKey, err := d.querier.CreateTeamDeleteKey(ctx, teamSlug, userID)
	if err != nil {
		return nil, err
	}
	return &TeamDeleteKey{TeamDeleteKey: deleteKey}, nil
}

func (d *database) GetTeamDeleteKey(ctx context.Context, key uuid.UUID) (*TeamDeleteKey, error) {
	deleteKey, err := d.querier.GetTeamDeleteKey(ctx, key)
	if err != nil {
		return nil, err
	}
	return &TeamDeleteKey{TeamDeleteKey: deleteKey}, nil
}

func (d *database) ConfirmTeamDeleteKey(ctx context.Context, key uuid.UUID) error {
	return d.querier.ConfirmTeamDeleteKey(ctx, key)
}

func (d *database) DeleteTeam(ctx context.Context, teamSlug slug.Slug) error {
	return d.querier.DeleteTeam(ctx, teamSlug)
}

func (d *database) GetTeamMemberOptOuts(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) ([]*gensql.GetTeamMemberOptOutsRow, error) {
	return d.querier.GetTeamMemberOptOuts(ctx, userID, teamSlug)
}

func (d *database) GetTeamsWithPermissionInGitHubRepo(ctx context.Context, repoName, permission string, offset, limit int) ([]*Team, int, error) {
	panic("not implemented")
	// 	matcher, err := json.Marshal(map[string]interface{}{
	// 		"repositories": []map[string]interface{}{
	// 			{
	// 				"name": repoName,
	// 				"permissions": []map[string]interface{}{
	// 					{
	// 						"name":    permission,
	// 						"granted": true,
	// 					},
	// 				},
	// 			},
	// 		},
	// 	})
	// 	if err != nil {
	// 		return nil, 0, err
	// 	}

	// 	rows, err := d.querier.GetTeamsWithPermissionInGitHubRepo(ctx, matcher, int32(offset), int32(limit))
	// 	if err != nil {
	// 		return nil, 0, err
	// 	}

	// 	teams := make([]*Team, 0)
	// 	for _, row := range rows {
	// 		teams = append(teams, &Team{Team: row})
	// 	}

	// 	total, err := d.querier.GetTeamsWithPermissionInGitHubRepoCount(ctx, matcher)
	// 	if err != nil {
	// 		return nil, 0, err
	// 	}

	// return teams, int(total), nil
}

func (d *database) SearchTeams(ctx context.Context, slugMatch string, limit int32) ([]*gensql.Team, error) {
	return d.querier.SearchTeams(ctx, slugMatch, limit)
}

func (d *database) TeamExists(ctx context.Context, team slug.Slug) (bool, error) {
	return d.querier.TeamExists(ctx, team)
}

func (d *database) SetGoogleGroupEmailForTeam(ctx context.Context, teamSlug slug.Slug, email string) error {
	return d.querier.SetGoogleGroupEmailForTeam(ctx, email, teamSlug)
}
