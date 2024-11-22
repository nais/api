package grpcteam

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/grpc/grpcpagination"
	"github.com/nais/api/internal/grpc/grpcteam/grpcteamsql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"
)

type Server struct {
	querier grpcteamsql.Querier
	protoapi.UnimplementedTeamsServer
}

func NewServer(pool *pgxpool.Pool) *Server {
	return &Server{
		querier: grpcteamsql.New(pool),
	}
}

func (t *Server) Delete(ctx context.Context, req *protoapi.DeleteTeamRequest) (*protoapi.DeleteTeamResponse, error) {
	if req.Slug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slug is required")
	}

	if err := t.querier.Delete(ctx, slug.Slug(req.Slug)); err != nil {
		return nil, status.Errorf(codes.Internal, "unable to delete team: %q", req.Slug)
	}

	return &protoapi.DeleteTeamResponse{}, nil
}

func (t *Server) Get(ctx context.Context, req *protoapi.GetTeamRequest) (*protoapi.GetTeamResponse, error) {
	team, err := t.querier.Get(ctx, slug.Slug(req.Slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "team not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team")
	}

	return &protoapi.GetTeamResponse{
		Team: toProtoTeam(team),
	}, nil
}

func (t *Server) List(ctx context.Context, req *protoapi.ListTeamsRequest) (*protoapi.ListTeamsResponse, error) {
	limit, offset := grpcpagination.Pagination(req)
	teams, err := t.querier.List(ctx, grpcteamsql.ListParams{
		Offset: offset,
		Limit:  limit,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list teams: %s", err)
	}

	total, err := t.querier.Count(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get teams count: %s", err)
	}

	resp := &protoapi.ListTeamsResponse{
		PageInfo: grpcpagination.PageInfo(req, int(total)),
		Nodes:    make([]*protoapi.Team, len(teams)),
	}
	for i, team := range teams {
		resp.Nodes[i] = toProtoTeam(team)
	}

	return resp, nil
}

func (t *Server) Members(ctx context.Context, req *protoapi.ListTeamMembersRequest) (*protoapi.ListTeamMembersResponse, error) {
	limit, offset := grpcpagination.Pagination(req)
	users, err := t.querier.ListMembers(ctx, grpcteamsql.ListMembersParams{
		TeamSlug: slug.Slug(req.Slug),
		Offset:   offset,
		Limit:    limit,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list team members: %s", err)
	}

	total, err := t.querier.CountMembers(ctx, slug.Slug(req.Slug))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team members count: %s", err)
	}

	resp := &protoapi.ListTeamMembersResponse{
		PageInfo: grpcpagination.PageInfo(req, int(total)),
		Nodes:    make([]*protoapi.TeamMember, len(users)),
	}
	for i, user := range users {
		resp.Nodes[i] = toProtoTeamMember(user)
	}

	return resp, nil
}

func (t *Server) SetTeamExternalReferences(ctx context.Context, req *protoapi.SetTeamExternalReferencesRequest) (*protoapi.SetTeamExternalReferencesResponse, error) {
	if req.Slug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slug is required")
	}

	var aID *uuid.UUID
	if req.EntraIdGroupId != nil {
		id, err := uuid.Parse(*req.EntraIdGroupId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "entra ID group ID must be a valid UUID: %s", err)
		}
		aID = &id
	}

	err := t.querier.UpdateExternalReferences(ctx, grpcteamsql.UpdateExternalReferencesParams{
		Slug:             slug.Slug(req.Slug),
		EntraIDGroupID:   aID,
		GithubTeamSlug:   req.GithubTeamSlug,
		GoogleGroupEmail: req.GoogleGroupEmail,
		GarRepository:    req.GarRepository,
		CdnBucket:        req.CdnBucket,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update external references for team: %s", err)
	}

	return &protoapi.SetTeamExternalReferencesResponse{}, nil
}

func (t *Server) SetTeamEnvironmentExternalReferences(ctx context.Context, req *protoapi.SetTeamEnvironmentExternalReferencesRequest) (*protoapi.SetTeamEnvironmentExternalReferencesResponse, error) {
	if req.Slug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "slug is required")
	}

	err := t.querier.UpsertEnvironment(ctx, grpcteamsql.UpsertEnvironmentParams{
		TeamSlug:     slug.Slug(req.Slug),
		Environment:  req.EnvironmentName,
		GcpProjectID: req.GcpProjectId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update external references for team: %s", err)
	}

	return &protoapi.SetTeamEnvironmentExternalReferencesResponse{}, nil
}

func (t *Server) Environments(ctx context.Context, req *protoapi.ListTeamEnvironmentsRequest) (*protoapi.ListTeamEnvironmentsResponse, error) {
	limit, offset := grpcpagination.Pagination(req)
	environments, err := t.querier.ListEnvironments(ctx, grpcteamsql.ListEnvironmentsParams{
		TeamSlug: slug.Slug(req.Slug),
		Offset:   offset,
		Limit:    limit,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list team environments: %s", err)
	}

	total, err := t.querier.CountEnvironments(ctx, slug.Slug(req.Slug))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team environments count: %s", err)
	}

	resp := &protoapi.ListTeamEnvironmentsResponse{
		PageInfo: grpcpagination.PageInfo(req, int(total)),
		Nodes:    make([]*protoapi.TeamEnvironment, len(environments)),
	}
	for i, env := range environments {
		resp.Nodes[i] = toProtoTeamEnvironment(env)
	}

	return resp, nil
}

func (t *Server) ListAuthorizedRepositories(ctx context.Context, req *protoapi.ListAuthorizedRepositoriesRequest) (*protoapi.ListAuthorizedRepositoriesResponse, error) {
	teamSlug := slug.Slug(req.TeamSlug)
	repositories, err := t.querier.GetTeamRepositories(ctx, teamSlug)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list repositories")
	}

	return &protoapi.ListAuthorizedRepositoriesResponse{
		GithubRepositories: repositories,
	}, nil
}

func (t *Server) IsRepositoryAuthorized(ctx context.Context, req *protoapi.IsRepositoryAuthorizedRequest) (*protoapi.IsRepositoryAuthorizedResponse, error) {
	authorized, err := t.querier.IsTeamRepository(ctx, grpcteamsql.IsTeamRepositoryParams{
		TeamSlug:         slug.Slug(req.TeamSlug),
		GithubRepository: req.Repository,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check repository authorization")
	}

	return &protoapi.IsRepositoryAuthorizedResponse{IsAuthorized: authorized}, nil
}

func toProtoTeam(team *grpcteamsql.Team) *protoapi.Team {
	var aID *string
	if team.EntraIDGroupID != nil {
		aID = ptr.To(team.EntraIDGroupID.String())
	}

	t := &protoapi.Team{
		Slug:             team.Slug.String(),
		Purpose:          team.Purpose,
		SlackChannel:     team.SlackChannel,
		EntraIdGroupId:   aID,
		GithubTeamSlug:   team.GithubTeamSlug,
		GoogleGroupEmail: team.GoogleGroupEmail,
		GarRepository:    team.GarRepository,
		CdnBucket:        team.CdnBucket,
	}

	if team.DeleteKeyConfirmedAt.Valid {
		t.DeleteKeyConfirmedAt = timestamppb.New(team.DeleteKeyConfirmedAt.Time)
	}

	return t
}

func toProtoTeamMember(u *grpcteamsql.User) *protoapi.TeamMember {
	return &protoapi.TeamMember{
		User: &protoapi.User{
			Id:         u.ID.String(),
			Name:       u.Name,
			Email:      u.Email,
			ExternalId: u.ExternalID,
		},
	}
}

func toProtoTeamEnvironment(env *grpcteamsql.TeamAllEnvironment) *protoapi.TeamEnvironment {
	return &protoapi.TeamEnvironment{
		Id:                 env.ID.String(),
		Slug:               env.TeamSlug.String(),
		EnvironmentName:    env.Environment,
		Gcp:                env.Gcp,
		GcpProjectId:       env.GcpProjectID,
		SlackAlertsChannel: env.SlackAlertsChannel,
	}
}
