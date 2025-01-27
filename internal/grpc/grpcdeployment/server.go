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
	"k8s.io/utils/ptr"
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
	exists, err := s.querier.TeamExists(ctx, slug.Slug(req.GetTeamSlug()))
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, status.Errorf(codes.NotFound, "team does not exist")
	}

	if req.GetEnvironmentName() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "environment is required")
	}

	var repoName *string
	if req.HasRepository() {
		repoName = ptr.To(req.GetRepository())
	}
	id, err := s.querier.CreateDeployment(ctx, grpcdeploymentsql.CreateDeploymentParams{
		CreatedAt: pgtype.Timestamptz{
			Time:  req.GetCreatedAt().AsTime(),
			Valid: req.GetCreatedAt().IsValid(),
		},
		TeamSlug:        slug.Slug(req.GetTeamSlug()),
		Repository:      repoName,
		EnvironmentName: req.GetEnvironmentName(),
	})
	if err != nil {
		return nil, err
	}

	return protoapi.CreateDeploymentResponse_builder{
		Id: ptr.To(id.String()),
	}.Build(), nil
}

func (s *Server) CreateDeploymentK8SResource(ctx context.Context, req *protoapi.CreateDeploymentK8SResourceRequest) (*protoapi.CreateDeploymentK8SResourceResponse, error) {
	uid, err := uuid.Parse(req.GetDeploymentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deployment id")
	}

	switch {
	case req.GetGroup() == "":
		return nil, status.Errorf(codes.InvalidArgument, "group is required")
	case req.GetVersion() == "":
		return nil, status.Errorf(codes.InvalidArgument, "version is required")
	case req.GetKind() == "":
		return nil, status.Errorf(codes.InvalidArgument, "kind is required")
	case req.GetName() == "":
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	case req.GetNamespace() == "":
		return nil, status.Errorf(codes.InvalidArgument, "namespace is required")
	}

	id, err := s.querier.CreateDeploymentK8sResource(ctx, grpcdeploymentsql.CreateDeploymentK8sResourceParams{
		DeploymentID: uid,
		CreatedAt: pgtype.Timestamptz{
			Time:  req.GetCreatedAt().AsTime(),
			Valid: req.GetCreatedAt().IsValid(),
		},
		Group:     req.GetGroup(),
		Version:   req.GetVersion(),
		Kind:      req.GetKind(),
		Name:      req.GetName(),
		Namespace: req.GetNamespace(),
	})
	if err != nil {
		return nil, err
	}

	return protoapi.CreateDeploymentK8SResourceResponse_builder{
		Id: ptr.To(id.String()),
	}.Build(), nil
}

func (s *Server) CreateDeploymentStatus(ctx context.Context, req *protoapi.CreateDeploymentStatusRequest) (*protoapi.CreateDeploymentStatusResponse, error) {
	uid, err := uuid.Parse(req.GetDeploymentId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deployment id")
	}

	if req.GetMessage() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "message is required")
	}

	state, ok := toSQLStateEnum(req.GetState())
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid state")
	}

	id, err := s.querier.CreateDeploymentStatus(ctx, grpcdeploymentsql.CreateDeploymentStatusParams{
		CreatedAt: pgtype.Timestamptz{
			Time:  req.GetCreatedAt().AsTime(),
			Valid: req.GetCreatedAt().IsValid(),
		},
		DeploymentID: uid,
		State:        state,
		Message:      req.GetMessage(),
	})
	if err != nil {
		return nil, err
	}

	return protoapi.CreateDeploymentStatusResponse_builder{
		Id: ptr.To(id.String()),
	}.Build(), nil
}

func toSQLStateEnum(gs protoapi.DeploymentState) (grpcdeploymentsql.DeploymentState, bool) {
	mapped := grpcdeploymentsql.DeploymentState(gs.String())
	if !slices.Contains(grpcdeploymentsql.AllDeploymentStateValues(), mapped) {
		return "", false
	}

	return mapped, true
}
