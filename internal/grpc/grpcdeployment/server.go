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
	querier                grpcdeploymentsql.Querier
	mappedEnvironmentNames map[string]string
	protoapi.UnimplementedDeploymentsServer
}

func NewServer(pool *pgxpool.Pool, mappedEnvironmentNames map[string]string) *Server {
	return &Server{
		querier:                grpcdeploymentsql.New(pool),
		mappedEnvironmentNames: mappedEnvironmentNames,
	}
}

func (s *Server) CreateDeployment(ctx context.Context, req *protoapi.CreateDeploymentRequest) (*protoapi.CreateDeploymentResponse, error) {
	if exists, err := s.querier.TeamExists(ctx, slug.Slug(req.GetTeamSlug())); err != nil {
		return nil, err
	} else if !exists {
		return nil, status.Errorf(codes.NotFound, "team does not exist")
	}

	if req.GetEnvironmentName() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "environment is required")
	}

	var repoName *string
	if req.HasRepository() {
		repoName = ptr.To(req.GetRepository())
	}

	var externalID *string
	if id := req.GetExternalId(); id != "" {
		externalID = ptr.To(id)
	}

	var commitSha *string
	if sha := req.GetCommitSha(); sha != "" {
		commitSha = ptr.To(sha)
	}

	var deployerUsername *string
	if username := req.GetDeployerUsername(); username != "" {
		deployerUsername = ptr.To(username)
	}

	var triggerUrl *string
	if url := req.GetTriggerUrl(); url != "" {
		triggerUrl = ptr.To(url)
	}

	id, err := s.querier.CreateDeployment(ctx, grpcdeploymentsql.CreateDeploymentParams{
		ExternalID: externalID,
		CreatedAt: pgtype.Timestamptz{
			Time:  req.GetCreatedAt().AsTime(),
			Valid: req.GetCreatedAt().IsValid(),
		},
		TeamSlug:         slug.Slug(req.GetTeamSlug()),
		Repository:       repoName,
		EnvironmentName:  s.mapEnvironmentName(req.GetEnvironmentName()),
		CommitSha:        commitSha,
		DeployerUsername: deployerUsername,
		TriggerUrl:       triggerUrl,
	})
	if err != nil {
		return nil, err
	}

	return protoapi.CreateDeploymentResponse_builder{
		Id: ptr.To(id.String()),
	}.Build(), nil
}

func (s *Server) CreateDeploymentK8SResource(ctx context.Context, req *protoapi.CreateDeploymentK8SResourceRequest) (*protoapi.CreateDeploymentK8SResourceResponse, error) {
	switch {
	case !req.HasGroup():
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

	var uid uuid.UUID
	var externalID *string
	switch req.WhichReference() {
	case protoapi.CreateDeploymentK8SResourceRequest_DeploymentId_case:
		var err error
		uid, err = uuid.Parse(req.GetDeploymentId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid deployment id")
		}
	case protoapi.CreateDeploymentK8SResourceRequest_ExternalDeploymentId_case:
		if req.GetExternalDeploymentId() == "" {
			return nil, status.Errorf(codes.InvalidArgument, "external deployment id cannot be empty")
		}
		externalID = ptr.To(req.GetExternalDeploymentId())
	default:
		return nil, status.Errorf(codes.InvalidArgument, "reference is required")
	}

	id, err := s.querier.CreateDeploymentK8sResource(ctx, grpcdeploymentsql.CreateDeploymentK8sResourceParams{
		DeploymentID:         uid,
		ExternalDeploymentID: externalID,
		Group:                req.GetGroup(),
		Version:              req.GetVersion(),
		Kind:                 req.GetKind(),
		Name:                 req.GetName(),
		Namespace:            req.GetNamespace(),
	})
	if err != nil {
		return nil, err
	}

	return protoapi.CreateDeploymentK8SResourceResponse_builder{
		Id: ptr.To(id.String()),
	}.Build(), nil
}

func (s *Server) CreateDeploymentStatus(ctx context.Context, req *protoapi.CreateDeploymentStatusRequest) (*protoapi.CreateDeploymentStatusResponse, error) {
	if req.GetMessage() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "message is required")
	} else if !req.HasState() {
		return nil, status.Errorf(codes.InvalidArgument, "state is required")
	}

	var uid uuid.UUID
	var externalID *string
	switch req.WhichReference() {
	case protoapi.CreateDeploymentStatusRequest_DeploymentId_case:
		var err error
		uid, err = uuid.Parse(req.GetDeploymentId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid deployment id")
		}
	case protoapi.CreateDeploymentStatusRequest_ExternalDeploymentId_case:
		if req.GetExternalDeploymentId() == "" {
			return nil, status.Errorf(codes.InvalidArgument, "external deployment id cannot be empty")
		}
		externalID = ptr.To(req.GetExternalDeploymentId())
	default:
		return nil, status.Errorf(codes.InvalidArgument, "reference is required")
	}

	state, ok := toSQLStateEnum(req.GetState())
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid state")
	}

	id, err := s.querier.CreateDeploymentStatus(ctx, grpcdeploymentsql.CreateDeploymentStatusParams{
		DeploymentID:         uid,
		ExternalDeploymentID: externalID,
		CreatedAt: pgtype.Timestamptz{
			Time:  req.GetCreatedAt().AsTime(),
			Valid: req.GetCreatedAt().IsValid(),
		},
		State:   state,
		Message: req.GetMessage(),
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

func (s *Server) mapEnvironmentName(name string) string {
	if s.mappedEnvironmentNames == nil {
		return name
	}

	if mapped, ok := s.mappedEnvironmentNames[name]; ok {
		return mapped
	}

	return name
}
