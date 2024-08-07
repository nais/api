package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"
)

type ReconcilersServer struct {
	db interface {
		database.ReconcilerRepo
		database.ReconcilerErrorRepo
		database.ReconcilerStateRepo
		database.TeamRepo
	}
	protoapi.UnimplementedReconcilersServer
}

func (s *ReconcilersServer) SetReconcilerErrorForTeam(ctx context.Context, req *protoapi.SetReconcilerErrorForTeamRequest) (*protoapi.SetReconcilerErrorForTeamResponse, error) {
	switch {
	case req.TeamSlug == "":
		return nil, status.Errorf(codes.InvalidArgument, "team slug is required")
	case req.ReconcilerName == "":
		return nil, status.Errorf(codes.InvalidArgument, "reconciler name is required")
	case req.ErrorMessage == "":
		return nil, status.Errorf(codes.InvalidArgument, "error message is required")
	case req.CorrelationId == "":
		return nil, status.Errorf(codes.InvalidArgument, "correlation id is required")
	}

	correlationID, err := uuid.Parse(req.CorrelationId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "correlation id is invalid")
	}

	if err := s.db.SetReconcilerErrorForTeam(ctx, correlationID, slug.Slug(req.TeamSlug), req.ReconcilerName, errors.New(req.ErrorMessage)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set reconciler error for team: %s", err)
	}

	return &protoapi.SetReconcilerErrorForTeamResponse{}, nil
}

func (s *ReconcilersServer) RemoveReconcilerErrorForTeam(ctx context.Context, req *protoapi.RemoveReconcilerErrorForTeamRequest) (*protoapi.RemoveReconcilerErrorForTeamResponse, error) {
	switch {
	case req.TeamSlug == "":
		return nil, status.Errorf(codes.InvalidArgument, "team slug is required")
	case req.ReconcilerName == "":
		return nil, status.Errorf(codes.InvalidArgument, "reconciler name is required")
	}

	if err := s.db.ClearReconcilerErrorsForTeam(ctx, slug.Slug(req.TeamSlug), req.ReconcilerName); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove reconciler error for team: %s", err)
	}

	return &protoapi.RemoveReconcilerErrorForTeamResponse{}, nil
}

func (s *ReconcilersServer) SuccessfulTeamSync(ctx context.Context, req *protoapi.SuccessfulTeamSyncRequest) (*protoapi.SuccessfulTeamSyncResponse, error) {
	if req.TeamSlug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "team slug is required")
	}

	if err := s.db.SetLastSuccessfulSyncForTeam(ctx, slug.Slug(req.TeamSlug)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set successful sync for team: %s", err)
	}

	return &protoapi.SuccessfulTeamSyncResponse{}, nil
}

func (s *ReconcilersServer) Register(ctx context.Context, req *protoapi.RegisterReconcilerRequest) (*protoapi.RegisterReconcilerResponse, error) {
	for _, rec := range req.Reconcilers {
		if _, err := s.db.UpsertReconciler(ctx, rec.Name, rec.DisplayName, rec.Description, rec.MemberAware, rec.EnableByDefault); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to register reconciler")
		}

		if err := s.db.SyncReconcilerConfig(ctx, rec.Name, rec.Config); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to sync reconciler config")
		}
	}

	return &protoapi.RegisterReconcilerResponse{}, nil
}

func (s *ReconcilersServer) Get(ctx context.Context, req *protoapi.GetReconcilerRequest) (*protoapi.GetReconcilerResponse, error) {
	rec, err := s.db.GetReconciler(ctx, req.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "reconciler not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get reconciler")
	}

	return &protoapi.GetReconcilerResponse{
		Reconciler: toProtoReconciler(rec),
	}, nil
}

func (s *ReconcilersServer) List(ctx context.Context, req *protoapi.ListReconcilersRequest) (*protoapi.ListReconcilersResponse, error) {
	limit, offset := pagination(req)
	recs, total, err := s.db.GetReconcilers(ctx, database.Page{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get reconcilers")
	}

	ret := make([]*protoapi.Reconciler, len(recs))
	for i, rec := range recs {
		ret[i] = toProtoReconciler(rec)
	}

	return &protoapi.ListReconcilersResponse{
		Nodes:    ret,
		PageInfo: pageInfo(req, total),
	}, nil
}

func (s *ReconcilersServer) Config(ctx context.Context, req *protoapi.ConfigReconcilerRequest) (*protoapi.ConfigReconcilerResponse, error) {
	cfg, err := s.db.GetReconcilerConfig(ctx, req.ReconcilerName, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get reconciler config")
	}

	ret := make([]*protoapi.ReconcilerConfig, len(cfg))
	for i, c := range cfg {
		ret[i] = &protoapi.ReconcilerConfig{
			Key:         c.Key,
			DisplayName: c.DisplayName,
			Description: c.Description,
			Value:       ptr.Deref(c.Value, ""),
			Secret:      c.Secret,
		}
	}

	return &protoapi.ConfigReconcilerResponse{
		Nodes: ret,
	}, nil
}

func (s *ReconcilersServer) SaveState(ctx context.Context, req *protoapi.SaveReconcilerStateRequest) (*protoapi.SaveReconcilerStateResponse, error) {
	switch {
	case req.ReconcilerName == "":
		return nil, status.Errorf(codes.InvalidArgument, "reconcilerName is required")
	case req.TeamSlug == "":
		return nil, status.Errorf(codes.InvalidArgument, "teamSlug is required")
	case len(req.Value) == 0:
		return nil, status.Errorf(codes.InvalidArgument, "state is required")
	}

	if _, err := s.db.UpsertReconcilerState(ctx, req.ReconcilerName, slug.Slug(req.TeamSlug), req.Value); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save reconciler state")
	}

	return &protoapi.SaveReconcilerStateResponse{}, nil
}

func (s *ReconcilersServer) State(ctx context.Context, req *protoapi.GetReconcilerStateRequest) (*protoapi.GetReconcilerStateResponse, error) {
	row, err := s.db.GetReconcilerStateForTeam(ctx, req.ReconcilerName, slug.Slug(req.TeamSlug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "state not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get reconciler state")
	}

	return &protoapi.GetReconcilerStateResponse{
		State: toProtoReconcilerState(row),
	}, nil
}

func (s *ReconcilersServer) DeleteState(ctx context.Context, req *protoapi.DeleteReconcilerStateRequest) (*protoapi.DeleteReconcilerStateResponse, error) {
	if req.ReconcilerName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "reconcilerName is required")
	}

	if req.TeamSlug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "teamSlug is required")
	}

	if err := s.db.DeleteReconcilerStateForTeam(ctx, req.ReconcilerName, slug.Slug(req.TeamSlug)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete reconciler state for team")
	}

	return &protoapi.DeleteReconcilerStateResponse{}, nil
}

func toProtoReconcilerState(res *database.ReconcilerState) *protoapi.ReconcilerState {
	return &protoapi.ReconcilerState{
		Id:             res.ID.String(),
		ReconcilerName: res.ReconcilerName,
		TeamSlug:       string(res.TeamSlug),
		Value:          res.Value,
		CreatedAt:      timestamppb.New(res.CreatedAt.Time),
		UpdatedAt:      timestamppb.New(res.UpdatedAt.Time),
	}
}

func toProtoReconciler(rec *database.Reconciler) *protoapi.Reconciler {
	return &protoapi.Reconciler{
		Name:        rec.Name,
		Description: rec.Description,
		DisplayName: rec.DisplayName,
		Enabled:     rec.Enabled,
		MemberAware: rec.MemberAware,
	}
}
