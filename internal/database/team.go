package database

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

const teamDeleteKeyLifetime = time.Hour * 1

type gitHubState struct {
	Repositories []*GitHubRepository `json:"repositories"`
}

type GitHubRepository struct {
	Name        string                        `json:"name"`
	Permissions []*GitHubRepositoryPermission `json:"permissions"`
	Archived    bool                          `json:"archived"`
	RoleName    string                        `json:"roleName"`
}

type GitHubRepositoryPermission struct {
	Name    string `json:"name"`
	Granted bool   `json:"granted"`
}

type TeamRepo interface {
	ConfirmTeamDeleteKey(ctx context.Context, key uuid.UUID) error
	CreateTeam(ctx context.Context, teamSlug slug.Slug, purpose, slackChannel string) (*Team, error)
	CreateTeamDeleteKey(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*TeamDeleteKey, error)
	DeleteTeam(ctx context.Context, teamSlug slug.Slug) error
	GetActiveTeamBySlug(ctx context.Context, teamSlug slug.Slug) (*Team, error)
	GetActiveTeams(ctx context.Context) ([]*Team, error)
	GetAllTeamMembers(ctx context.Context, teamSlug slug.Slug) ([]*User, error)
	GetTeamBySlug(ctx context.Context, teamSlug slug.Slug) (*Team, error)
	GetTeamDeleteKey(ctx context.Context, key uuid.UUID) (*TeamDeleteKey, error)
	GetTeamEnvironments(ctx context.Context, teamSlug slug.Slug, p Page) ([]*TeamEnvironment, int, error)
	GetTeamEnvironmentsBySlugsAndEnvNames(ctx context.Context, keys []EnvSlugName) ([]*TeamEnvironment, error)
	GetTeamMember(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*User, error)
	GetTeamMemberOptOuts(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) ([]*gensql.GetTeamMemberOptOutsRow, error)
	GetTeamMembers(ctx context.Context, teamSlug slug.Slug, p Page) ([]*User, int, error)
	GetTeamMembersForReconciler(ctx context.Context, teamSlug slug.Slug, reconcilerName string) ([]*User, error)
	GetTeams(ctx context.Context, p Page) ([]*Team, int, error)
	GetTeamsBySlugs(ctx context.Context, teamSlugs []slug.Slug) ([]*Team, error)
	GetTeamsWithPermissionInGitHubRepo(ctx context.Context, repoName, permission string, p Page) ([]*Team, int, error)
	GetUserTeams(ctx context.Context, userID uuid.UUID, p Page) ([]*UserTeam, int, error)
	RemoveUserFromTeam(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug) error
	SearchTeams(ctx context.Context, slugMatch string, limit int32) ([]*gensql.Team, error)
	SetLastSuccessfulSyncForTeam(ctx context.Context, teamSlug slug.Slug) error
	TeamExists(ctx context.Context, team slug.Slug) (bool, error)
	UpdateTeam(ctx context.Context, teamSlug slug.Slug, purpose, slackChannel *string) (*Team, error)
	UpdateTeamExternalReferences(ctx context.Context, params gensql.UpdateTeamExternalReferencesParams) (*Team, error)
	UpsertTeamEnvironment(ctx context.Context, teamSlug slug.Slug, environment string, slackChannel, gcpProjectID *string) error
}

var _ TeamRepo = &database{}

type EnvSlugName struct {
	Slug    slug.Slug
	EnvName string
}

type TeamEnvironment struct {
	*gensql.TeamAllEnvironment
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
	return d.querier.RemoveUserFromTeam(ctx, gensql.RemoveUserFromTeamParams{
		UserID:   userID,
		TeamSlug: teamSlug,
	})
}

func (d *database) UpdateTeam(ctx context.Context, teamSlug slug.Slug, purpose, slackChannel *string) (*Team, error) {
	team, err := d.querier.UpdateTeam(ctx, gensql.UpdateTeamParams{
		Purpose:      purpose,
		SlackChannel: slackChannel,
		Slug:         teamSlug,
	})
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) UpdateTeamExternalReferences(ctx context.Context, params gensql.UpdateTeamExternalReferencesParams) (*Team, error) {
	team, err := d.querier.UpdateTeamExternalReferences(ctx, params)
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) CreateTeam(ctx context.Context, teamSlug slug.Slug, purpose, slackChannel string) (*Team, error) {
	team, err := d.querier.CreateTeam(ctx, gensql.CreateTeamParams{
		Slug:         teamSlug,
		Purpose:      purpose,
		SlackChannel: slackChannel,
	})
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) GetActiveTeamBySlug(ctx context.Context, teamSlug slug.Slug) (*Team, error) {
	team, err := d.querier.GetActiveTeamBySlug(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) GetTeamBySlug(ctx context.Context, teamSlug slug.Slug) (*Team, error) {
	team, err := d.querier.GetTeamBySlug(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	return &Team{Team: team}, nil
}

func (d *database) GetTeams(ctx context.Context, p Page) ([]*Team, int, error) {
	var teams []*gensql.Team
	var err error
	teams, err = d.querier.GetTeams(ctx, gensql.GetTeamsParams{
		Offset: int32(p.Offset),
		Limit:  int32(p.Limit),
	})
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

func (d *database) GetTeamsBySlugs(ctx context.Context, teamSlugs []slug.Slug) ([]*Team, error) {
	teams, err := d.querier.GetTeamBySlugs(ctx, teamSlugs)
	if err != nil {
		return nil, err
	}

	collection := make([]*Team, 0)
	for _, team := range teams {
		collection = append(collection, &Team{Team: team})
	}

	return collection, nil
}

func (d *database) GetTeamEnvironments(ctx context.Context, teamSlug slug.Slug, p Page) ([]*TeamEnvironment, int, error) {
	rows, err := d.querier.GetTeamEnvironments(ctx, gensql.GetTeamEnvironmentsParams{
		TeamSlug: teamSlug,
		Offset:   int32(p.Offset),
		Limit:    int32(p.Limit),
	})
	if err != nil {
		return nil, 0, err
	}

	envs := make([]*TeamEnvironment, len(rows))
	for i, row := range rows {
		envs[i] = &TeamEnvironment{TeamAllEnvironment: row}
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

func (d *database) GetUserTeams(ctx context.Context, userID uuid.UUID, p Page) ([]*UserTeam, int, error) {
	rows, err := d.querier.GetUserTeams(ctx, gensql.GetUserTeamsParams{
		UserID: userID,
		Offset: int32(p.Offset),
		Limit:  int32(p.Limit),
	})
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

func (d *database) GetTeamMembers(ctx context.Context, teamSlug slug.Slug, p Page) ([]*User, int, error) {
	var rows []*gensql.User
	var err error
	rows, err = d.querier.GetTeamMembers(ctx, gensql.GetTeamMembersParams{
		TeamSlug: teamSlug,
		Offset:   int32(p.Offset),
		Limit:    int32(p.Limit),
	})
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
	user, err := d.querier.GetTeamMember(ctx, gensql.GetTeamMemberParams{
		TeamSlug: teamSlug,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}

	return &User{User: user}, nil
}

func (d *database) GetTeamMembersForReconciler(ctx context.Context, teamSlug slug.Slug, reconcilerName string) ([]*User, error) {
	rows, err := d.querier.GetTeamMembersForReconciler(ctx, gensql.GetTeamMembersForReconcilerParams{
		TeamSlug:       teamSlug,
		ReconcilerName: reconcilerName,
	})
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

func (d *database) CreateTeamDeleteKey(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (*TeamDeleteKey, error) {
	deleteKey, err := d.querier.CreateTeamDeleteKey(ctx, gensql.CreateTeamDeleteKeyParams{
		TeamSlug:  teamSlug,
		CreatedBy: userID,
	})
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
	return d.querier.GetTeamMemberOptOuts(ctx, gensql.GetTeamMemberOptOutsParams{
		UserID:   userID,
		TeamSlug: teamSlug,
	})
}

func (d *database) GetTeamEnvironmentsBySlugsAndEnvNames(ctx context.Context, keys []EnvSlugName) ([]*TeamEnvironment, error) {
	teamSlugs := make([]slug.Slug, len(keys))
	envNames := make([]string, len(keys))
	for i, key := range keys {
		teamSlugs[i] = key.Slug
		envNames[i] = key.EnvName
	}

	ret, err := d.querier.GetTeamEnvironmentsBySlugsAndEnvNames(ctx, gensql.GetTeamEnvironmentsBySlugsAndEnvNamesParams{
		TeamSlugs:    teamSlugs,
		Environments: envNames,
	})
	if err != nil {
		return nil, err
	}

	envs := make([]*TeamEnvironment, len(ret))
	for i, row := range ret {
		envs[i] = &TeamEnvironment{TeamAllEnvironment: row}
	}

	return envs, nil
}

func (d *database) UpsertTeamEnvironment(ctx context.Context, teamSlug slug.Slug, environment string, slackChannel, gcpProjectID *string) error {
	_, err := d.querier.UpsertTeamEnvironment(ctx, gensql.UpsertTeamEnvironmentParams{
		TeamSlug:           teamSlug,
		Environment:        environment,
		SlackAlertsChannel: slackChannel,
		GcpProjectID:       gcpProjectID,
	})

	return err
}

func (d *database) GetTeamsWithPermissionInGitHubRepo(ctx context.Context, repoName, permission string, p Page) ([]*Team, int, error) {
	// TODO: this should be refactored once we have a better model for the github reconciler state

	states, err := d.GetReconcilerState(ctx, "github:team")
	if err != nil {
		return nil, 0, err
	}

	teams := make([]*Team, 0)
	for _, state := range states {
		if hasRepoWithPermission(state.ReconcilerState.Value, repoName, permission) {
			teams = append(teams, state.Team)
		}
	}

	start := p.Offset
	end := start + p.Limit
	total := len(teams)
	if end > total {
		end = total
	}

	return teams[start:end], total, nil
}

func (d *database) SearchTeams(ctx context.Context, slugMatch string, limit int32) ([]*gensql.Team, error) {
	return d.querier.SearchTeams(ctx, gensql.SearchTeamsParams{
		SlugMatch: slugMatch,
		Limit:     limit,
	})
}

func (d *database) TeamExists(ctx context.Context, team slug.Slug) (bool, error) {
	return d.querier.TeamExists(ctx, team)
}

func GetGitHubRepos(b []byte) ([]*GitHubRepository, error) {
	var state gitHubState
	err := json.Unmarshal(b, &state)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(state.Repositories, func(i, j int) bool {
		return state.Repositories[i].Name < state.Repositories[j].Name
	})
	return state.Repositories, nil
}

func hasRepoWithPermission(b []byte, repoName, permission string) bool {
	repos, err := GetGitHubRepos(b)
	if err != nil {
		return false
	}

	for _, repo := range repos {
		if repo.Name != repoName {
			continue
		}

		for _, perm := range repo.Permissions {
			if perm.Name == permission && perm.Granted {
				return true
			}
		}
	}

	return false
}
