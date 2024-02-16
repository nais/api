package grpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"
)

type TeamsServer struct {
	db interface {
		database.TeamRepo
		database.RepositoryAuthorizationRepo
	}

	protoapi.UnimplementedTeamsServer
}

func (t *TeamsServer) Delete(ctx context.Context, req *protoapi.DeleteTeamRequest) (*protoapi.DeleteTeamResponse, error) {
	if req.Slug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slug is required")
	}

	if err := t.db.DeleteTeam(ctx, slug.Slug(req.Slug)); err != nil {
		return nil, status.Errorf(codes.Internal, "unable to delete team: %q", req.Slug)
	}

	return &protoapi.DeleteTeamResponse{}, nil
}

func (t *TeamsServer) Get(ctx context.Context, req *protoapi.GetTeamRequest) (*protoapi.GetTeamResponse, error) {
	team, err := t.db.GetTeamBySlug(ctx, slug.Slug(req.Slug))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "team not found")
	}

	return &protoapi.GetTeamResponse{
		Team: toProtoTeam(team),
	}, nil
}

func (t *TeamsServer) List(ctx context.Context, req *protoapi.ListTeamsRequest) (*protoapi.ListTeamsResponse, error) {
	limit, offset := pagination(req)
	teams, total, err := t.db.GetTeams(ctx, database.Page{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list teams: %s", err)
	}

	resp := &protoapi.ListTeamsResponse{
		PageInfo: pageInfo(req, total),
	}
	for _, team := range teams {
		resp.Nodes = append(resp.Nodes, toProtoTeam(team))
	}

	return resp, nil
}

func (t *TeamsServer) Members(ctx context.Context, req *protoapi.ListTeamMembersRequest) (*protoapi.ListTeamMembersResponse, error) {
	limit, offset := pagination(req)
	users, total, err := t.db.GetTeamMembers(ctx, slug.Slug(req.Slug), database.Page{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list team members: %s", err)
	}

	resp := &protoapi.ListTeamMembersResponse{
		PageInfo: pageInfo(req, total),
	}
	for _, user := range users {
		resp.Nodes = append(resp.Nodes, toProtoTeamMember(user))
	}

	return resp, nil
}

func (t *TeamsServer) SetTeamExternalReferences(ctx context.Context, req *protoapi.SetTeamExternalReferencesRequest) (*protoapi.SetTeamExternalReferencesResponse, error) {
	if req.Slug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slug is required")
	}

	var aID *uuid.UUID
	if req.AzureGroupId != nil {
		id, err := uuid.Parse(*req.AzureGroupId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "azure group ID must be a valid UUID: %s", err)
		}
		aID = &id
	}

	_, err := t.db.UpdateTeamExternalReferences(ctx, gensql.UpdateTeamExternalReferencesParams{
		Slug:             slug.Slug(req.Slug),
		AzureGroupID:     aID,
		GithubTeamSlug:   req.GithubTeamSlug,
		GoogleGroupEmail: req.GoogleGroupEmail,
		GarRepository:    req.GarRepository,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update external references for team: %s", err)
	}

	return &protoapi.SetTeamExternalReferencesResponse{}, nil
}

func (t *TeamsServer) SetTeamEnvironmentExternalReferences(ctx context.Context, req *protoapi.SetTeamEnvironmentExternalReferencesRequest) (*protoapi.SetTeamEnvironmentExternalReferencesResponse, error) {
	if req.Slug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slug is required")
	}

	err := t.db.UpsertTeamEnvironment(ctx, slug.Slug(req.Slug), req.EnvironmentName, nil, req.GcpProjectId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update external references for team: %s", err)
	}

	return &protoapi.SetTeamEnvironmentExternalReferencesResponse{}, nil
}

func (t *TeamsServer) Environments(ctx context.Context, req *protoapi.ListTeamEnvironmentsRequest) (*protoapi.ListTeamEnvironmentsResponse, error) {
	limit, offset := pagination(req)
	environments, total, err := t.db.GetTeamEnvironments(ctx, slug.Slug(req.Slug), database.Page{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list team environments: %s", err)
	}

	resp := &protoapi.ListTeamEnvironmentsResponse{
		PageInfo: pageInfo(req, total),
	}
	for _, env := range environments {
		resp.Nodes = append(resp.Nodes, toProtoTeamEnvironment(env))
	}

	return resp, nil
}

func (t *TeamsServer) ListAuthorizedRepositories(ctx context.Context, req *protoapi.ListAuthorizedRepositoriesRequest) (*protoapi.ListAuthorizedRepositoriesResponse, error) {
	teamSlug := slug.Slug(req.TeamSlug)
	repositories, err := t.db.ListRepositoriesByAuthorization(ctx, teamSlug, gensql.RepositoryAuthorizationEnumDeploy)
	if err != nil {
		return nil, fmt.Errorf("list repositories by authorization: %w", err)
	}

	return &protoapi.ListAuthorizedRepositoriesResponse{
		GithubRepositories: repositories,
	}, nil
}

func toProtoTeam(team *database.Team) *protoapi.Team {
	var aID *string
	if team.AzureGroupID != nil {
		aID = ptr.To(team.AzureGroupID.String())
	}

	return &protoapi.Team{
		Slug:             team.Slug.String(),
		Purpose:          team.Purpose,
		SlackChannel:     team.SlackChannel,
		AzureGroupId:     aID,
		GithubTeamSlug:   team.GithubTeamSlug,
		GoogleGroupEmail: team.GoogleGroupEmail,
		GarRepository:    team.GarRepository,
	}
}

func toProtoTeamMember(user *database.User) *protoapi.TeamMember {
	return &protoapi.TeamMember{
		User: toProtoUser(user),
	}
}

func toProtoTeamEnvironment(env *database.TeamEnvironment) *protoapi.TeamEnvironment {
	return &protoapi.TeamEnvironment{
		Id:                 env.ID.String(),
		Slug:               env.TeamSlug.String(),
		EnvironmentName:    env.Environment,
		Gcp:                env.Gcp,
		GcpProjectId:       env.GcpProjectID,
		SlackAlertsChannel: env.SlackAlertsChannel,
	}
}
