package grpcdeployment

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/grpc/grpcdeployment/grpcdeploymentsql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	querier grpcdeploymentsql.Querier
	protoapi.UnimplementedDeploymentsServer
}

func NewServer(pool *pgxpool.Pool) *Server {
	return &Server{
		querier: grpcdeploymentsql.New(pool),
	}
}

func (s *Server) CreateDeployment(ctx context.Context, req *protoapi.CreateDeploymentRequest) (*protoapi.CreateDeploymentResponse, error) {
	exists, err := s.querier.TeamExists(ctx, slug.Slug(req.TeamSlug))
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, status.Errorf(codes.NotFound, "team does not exist")
	}

	if req.Environment == "" {
		return nil, status.Errorf(codes.InvalidArgument, "environment is required")
	}

	id, err := s.querier.CreateDeployment(ctx, grpcdeploymentsql.CreateDeploymentParams{
		CreatedAt: pgtype.Timestamptz{
			Time:  req.CreatedAt.AsTime(),
			Valid: req.CreatedAt.IsValid(),
		},
		TeamSlug:        slug.Slug(req.TeamSlug),
		Repository:      req.Repository,
		EnvironmentName: req.Environment,
	})
	if err != nil {
		return nil, err
	}

	return &protoapi.CreateDeploymentResponse{
		Id: id.String(),
	}, nil
}

func (s *Server) CreateDeploymentK8SResource(ctx context.Context, req *protoapi.CreateDeploymentK8SResourceRequest) (*protoapi.CreateDeploymentK8SResourceResponse, error) {
	uid, err := uuid.Parse(req.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deployment id")
	}

	switch {
	case req.Group == "":
		return nil, status.Errorf(codes.InvalidArgument, "group is required")
	case req.Version == "":
		return nil, status.Errorf(codes.InvalidArgument, "version is required")
	case req.Kind == "":
		return nil, status.Errorf(codes.InvalidArgument, "kind is required")
	case req.Name == "":
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	case req.Namespace == "":
		return nil, status.Errorf(codes.InvalidArgument, "namespace is required")
	}

	id, err := s.querier.CreateDeploymentK8sResource(ctx, grpcdeploymentsql.CreateDeploymentK8sResourceParams{
		DeploymentID: uid,
		CreatedAt: pgtype.Timestamptz{
			Time:  req.CreatedAt.AsTime(),
			Valid: req.CreatedAt.IsValid(),
		},
		Group:     req.Group,
		Version:   req.Version,
		Kind:      req.Kind,
		Name:      req.Name,
		Namespace: req.Namespace,
	})
	if err != nil {
		return nil, err
	}

	return &protoapi.CreateDeploymentK8SResourceResponse{
		Id: id.String(),
	}, nil
}

func (s *Server) CreateDeploymentStatus(ctx context.Context, req *protoapi.CreateDeploymentStatusRequest) (*protoapi.CreateDeploymentStatusResponse, error) {
	uid, err := uuid.Parse(req.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deployment id")
	}

	if req.Message == "" {
		return nil, status.Errorf(codes.InvalidArgument, "message is required")
	}

	state, ok := toSQLStateEnum(req.State)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid state")
	}

	id, err := s.querier.CreateDeploymentStatus(ctx, grpcdeploymentsql.CreateDeploymentStatusParams{
		CreatedAt: pgtype.Timestamptz{
			Time:  req.CreatedAt.AsTime(),
			Valid: req.CreatedAt.IsValid(),
		},
		DeploymentID: uid,
		State:        state,
		Message:      req.Message,
	})
	if err != nil {
		return nil, err
	}

	return &protoapi.CreateDeploymentStatusResponse{
		Id: id.String(),
	}, nil
}

func toSQLStateEnum(gs protoapi.DeploymentState) (grpcdeploymentsql.DeploymentState, bool) {
	mapped := grpcdeploymentsql.DeploymentState(gs.String())
	if !slices.Contains(grpcdeploymentsql.AllDeploymentStateValues(), mapped) {
		return grpcdeploymentsql.DeploymentState(""), false
	}

	return mapped, true
}
