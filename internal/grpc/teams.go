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

func toProtoTeam(team *database.Team) *protoapi.Team {
	return &protoapi.Team{
		Slug:    team.Slug.String(),
		Purpose: team.Purpose,
	}
}
