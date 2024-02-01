package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TeamsServer struct {
	db database.TeamRepo
	protoapi.UnimplementedTeamsServer
}

func (t *TeamsServer) Get(ctx context.Context, r *protoapi.GetTeamRequest) (*protoapi.GetTeamResponse, error) {
	team, err := t.db.GetTeamBySlug(ctx, slug.Slug(r.Slug))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "team not found")
	}

	return &protoapi.GetTeamResponse{
		Team: toProtoTeam(team),
	}, nil
}

func (t *TeamsServer) List(ctx context.Context, r *protoapi.ListTeamsRequest) (*protoapi.ListTeamsResponse, error) {
	limit, offset := pagination(r)
	teams, total, err := t.db.GetTeams(ctx, database.Page{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list teams: %s", err)
	}

	resp := &protoapi.ListTeamsResponse{
		PageInfo: pageInfo(r, total),
	}
	for _, team := range teams {
		resp.Nodes = append(resp.Nodes, toProtoTeam(team))
	}

	return resp, nil
}

func (t *TeamsServer) Members(ctx context.Context, r *protoapi.ListTeamMembersRequest) (*protoapi.ListTeamMembersResponse, error) {
	limit, offset := pagination(r)
	users, total, err := t.db.GetTeamMembers(ctx, slug.Slug(r.Slug), database.Page{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list team members: %s", err)
	}

	resp := &protoapi.ListTeamMembersResponse{
		PageInfo: pageInfo(r, total),
	}
	for _, user := range users {
		resp.Nodes = append(resp.Nodes, toProtoTeamMember(user))
	}

	return resp, nil
}

func (t *TeamsServer) SlackAlertsChannels(ctx context.Context, r *protoapi.SlackAlertsChannelsRequest) (*protoapi.SlackAlertsChannelsResponse, error) {
	// TODO: Pagination?
	channelMap, err := t.db.GetSlackAlertsChannels(ctx, slug.Slug(r.Slug))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list slack alerts channels: %s", err)
	}

	resp := &protoapi.SlackAlertsChannelsResponse{}
	for env, name := range channelMap {
		resp.Channels = append(resp.Channels, &protoapi.SlackAlertsChannel{
			Environment: env,
			Channel:     name,
		})
	}

	return resp, nil
}

func (t *TeamsServer) SetTeamExternalReferences(ctx context.Context, r *protoapi.SetTeamExternalReferencesRequest) (*protoapi.SetTeamExternalReferencesResponse, error) {
	if r.Slug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slug is required")
	}

	var aID *uuid.UUID
	if r.AzureGroupId != nil {
		id, err := uuid.Parse(*r.AzureGroupId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "azure group ID must be a valid UUID: %s", err)
		}
		aID = &id
	}

	_, err := t.db.UpdateTeamExternalReferences(ctx, gensql.UpdateTeamExternalReferencesParams{
		Slug:             slug.Slug(r.Slug),
		AzureGroupID:     aID,
		GithubTeamSlug:   r.GithubTeamSlug,
		GoogleGroupEmail: r.GoogleGroupEmail,
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update external references for team: %s", err)
	}

	return &protoapi.SetTeamExternalReferencesResponse{}, nil
}

func (t *TeamsServer) Environments(ctx context.Context, r *protoapi.ListTeamEnvironmentsRequest) (*protoapi.ListTeamEnvironmentsResponse, error) {
	limit, offset := pagination(r)
	environments, total, err := t.db.GetTeamEnvironments(ctx, slug.Slug(r.Slug), database.Page{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list team environments: %s", err)
	}

	resp := &protoapi.ListTeamEnvironmentsResponse{
		PageInfo: pageInfo(r, total),
	}
	for _, env := range environments {
		resp.Nodes = append(resp.Nodes, toProtoTeamEnvironment(env))
	}

	return resp, nil
}

func toProtoTeam(team *database.Team) *protoapi.Team {
	gge := ""
	if team.GoogleGroupEmail != nil {
		gge = *team.GoogleGroupEmail
	}

	gts := ""
	if team.GithubTeamSlug != nil {
		gts = *team.GithubTeamSlug
	}

	aID := ""
	if team.AzureGroupID != nil {
		aID = team.AzureGroupID.String()
	}

	return &protoapi.Team{
		Slug:             team.Slug.String(),
		Purpose:          team.Purpose,
		SlackChannel:     team.SlackChannel,
		AzureGroupId:     aID,
		GithubTeamSlug:   gts,
		GoogleGroupEmail: gge,
	}
}

func toProtoTeamMember(user *database.User) *protoapi.TeamMember {
	return &protoapi.TeamMember{
		User: toProtoUser(user),
		// TODO: Role:   ...,
	}
}

func toProtoTeamEnvironment(env *database.TeamEnvironment) *protoapi.TeamEnvironment {
	return &protoapi.TeamEnvironment{
		Id:              env.ID.String(),
		Slug:            env.TeamSlug.String(),
		EnvironmentName: env.Environment,
		Namespace:       env.Namespace,
		GcpProjectId:    env.GcpProjectID,
	}
}
