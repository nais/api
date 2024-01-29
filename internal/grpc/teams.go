package grpc

import (
	"context"

	"github.com/nais/api/internal/database"
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
	teams, total, err := t.db.GetTeams(ctx, offset, limit)
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
	users, total, err := t.db.GetTeamMembers(ctx, slug.Slug(r.Slug), offset, limit)
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

func (t *TeamsServer) SetGoogleGroupEmailForTeam(ctx context.Context, r *protoapi.SetGoogleGroupEmailForTeamRequest) (*protoapi.SetGoogleGroupEmailForTeamResponse, error) {
	if r.Slug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slug is required")
	}

	if r.GoogleGroupEmail == "" {
		return nil, status.Errorf(codes.InvalidArgument, "google group email is required")
	}

	if err := t.db.SetGoogleGroupEmailForTeam(ctx, slug.Slug(r.Slug), r.GoogleGroupEmail); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set google group email for team: %s", err)
	}

	return &protoapi.SetGoogleGroupEmailForTeamResponse{}, nil
}

func (t *TeamsServer) Environments(ctx context.Context, r *protoapi.ListTeamEnvironmentsRequest) (*protoapi.ListTeamEnvironmentsResponse, error) {
	limit, offset := pagination(r)
	environments, total, err := t.db.GetTeamEnvironments(ctx, slug.Slug(r.Slug), offset, limit)
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
	return &protoapi.Team{
		Slug:             team.Slug.String(),
		Purpose:          team.Purpose,
		SlackChannel:     team.SlackChannel,
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
