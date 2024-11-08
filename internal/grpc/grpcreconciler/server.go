package grpcreconciler

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/grpc/grpcpagination"
	"github.com/nais/api/internal/grpc/grpcreconciler/grpcreconcilersql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"
)

type Server struct {
	querier grpcreconcilersql.Querier
	protoapi.UnimplementedReconcilersServer
}

func NewServer(querier grpcreconcilersql.Querier) *Server {
	return &Server{
		querier: querier,
	}
}

func (s *Server) SetReconcilerErrorForTeam(ctx context.Context, req *protoapi.SetReconcilerErrorForTeamRequest) (*protoapi.SetReconcilerErrorForTeamResponse, error) {
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

	if err := s.querier.SetErrorForTeam(ctx, grpcreconcilersql.SetErrorForTeamParams{
		CorrelationID: correlationID,
		TeamSlug:      slug.Slug(req.TeamSlug),
		Reconciler:    req.ReconcilerName,
		ErrorMessage:  req.ErrorMessage,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set reconciler error for team: %s", err)
	}

	return &protoapi.SetReconcilerErrorForTeamResponse{}, nil
}

func (s *Server) RemoveReconcilerErrorForTeam(ctx context.Context, req *protoapi.RemoveReconcilerErrorForTeamRequest) (*protoapi.RemoveReconcilerErrorForTeamResponse, error) {
	switch {
	case req.TeamSlug == "":
		return nil, status.Errorf(codes.InvalidArgument, "team slug is required")
	case req.ReconcilerName == "":
		return nil, status.Errorf(codes.InvalidArgument, "reconciler name is required")
	}

	if err := s.querier.ClearErrorsForTeam(ctx, grpcreconcilersql.ClearErrorsForTeamParams{
		TeamSlug:   slug.Slug(req.TeamSlug),
		Reconciler: req.ReconcilerName,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove reconciler error for team: %s", err)
	}

	return &protoapi.RemoveReconcilerErrorForTeamResponse{}, nil
}

func (s *Server) SuccessfulTeamSync(ctx context.Context, req *protoapi.SuccessfulTeamSyncRequest) (*protoapi.SuccessfulTeamSyncResponse, error) {
	if req.TeamSlug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "team slug is required")
	}

	if err := s.querier.SetLastSuccessfulSyncForTeam(ctx, slug.Slug(req.TeamSlug)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set successful sync for team: %s", err)
	}

	return &protoapi.SuccessfulTeamSyncResponse{}, nil
}

func (s *Server) Register(ctx context.Context, req *protoapi.RegisterReconcilerRequest) (*protoapi.RegisterReconcilerResponse, error) {
	for _, rec := range req.Reconcilers {
		if _, err := s.querier.Upsert(ctx, grpcreconcilersql.UpsertParams{
			Name:         rec.Name,
			DisplayName:  rec.DisplayName,
			Description:  rec.Description,
			MemberAware:  rec.MemberAware,
			EnabledIfNew: rec.EnableByDefault,
		}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to register reconciler")
		}

		if err := s.syncReconcilerConfig(ctx, rec.Name, rec.Config); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to sync reconciler config")
		}
	}

	return &protoapi.RegisterReconcilerResponse{}, nil
}

func (s *Server) Get(ctx context.Context, req *protoapi.GetReconcilerRequest) (*protoapi.GetReconcilerResponse, error) {
	rec, err := s.querier.Get(ctx, req.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "reconciler not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get reconciler")
	}

	return &protoapi.GetReconcilerResponse{
		Reconciler: toProtoReconciler(rec),
	}, nil
}

func (s *Server) List(ctx context.Context, req *protoapi.ListReconcilersRequest) (*protoapi.ListReconcilersResponse, error) {
	limit, offset := grpcpagination.Pagination(req)
	recs, err := s.querier.List(ctx, grpcreconcilersql.ListParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get reconcilers")
	}

	total, err := s.querier.Count(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to count reconcilers")
	}

	ret := make([]*protoapi.Reconciler, len(recs))
	for i, rec := range recs {
		ret[i] = toProtoReconciler(rec)
	}

	return &protoapi.ListReconcilersResponse{
		Nodes:    ret,
		PageInfo: grpcpagination.PageInfo(req, int(total)),
	}, nil
}

func (s *Server) Config(ctx context.Context, req *protoapi.ConfigReconcilerRequest) (*protoapi.ConfigReconcilerResponse, error) {
	cfg, err := s.querier.GetConfig(ctx, grpcreconcilersql.GetConfigParams{
		IncludeSecret:  true,
		ReconcilerName: req.ReconcilerName,
	})
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

func (s *Server) SaveState(ctx context.Context, req *protoapi.SaveReconcilerStateRequest) (*protoapi.SaveReconcilerStateResponse, error) {
	switch {
	case req.ReconcilerName == "":
		return nil, status.Errorf(codes.InvalidArgument, "reconcilerName is required")
	case req.TeamSlug == "":
		return nil, status.Errorf(codes.InvalidArgument, "teamSlug is required")
	case len(req.Value) == 0:
		return nil, status.Errorf(codes.InvalidArgument, "state is required")
	}

	if _, err := s.querier.UpsertState(ctx, grpcreconcilersql.UpsertStateParams{
		ReconcilerName: req.ReconcilerName,
		TeamSlug:       slug.Slug(req.TeamSlug),
		Value:          req.Value,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save reconciler state")
	}

	return &protoapi.SaveReconcilerStateResponse{}, nil
}

func (s *Server) State(ctx context.Context, req *protoapi.GetReconcilerStateRequest) (*protoapi.GetReconcilerStateResponse, error) {
	row, err := s.querier.GetStateForTeam(ctx, grpcreconcilersql.GetStateForTeamParams{
		ReconcilerName: req.ReconcilerName,
		TeamSlug:       slug.Slug(req.TeamSlug),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "state not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get reconciler state")
	}

	return &protoapi.GetReconcilerStateResponse{
		State: toProtoReconcilerState(row),
	}, nil
}

func (s *Server) DeleteState(ctx context.Context, req *protoapi.DeleteReconcilerStateRequest) (*protoapi.DeleteReconcilerStateResponse, error) {
	if req.ReconcilerName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "reconcilerName is required")
	}

	if req.TeamSlug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "teamSlug is required")
	}

	if err := s.querier.DeleteStateForTeam(ctx, grpcreconcilersql.DeleteStateForTeamParams{
		ReconcilerName: req.ReconcilerName,
		TeamSlug:       slug.Slug(req.TeamSlug),
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete reconciler state for team")
	}

	return &protoapi.DeleteReconcilerStateResponse{}, nil
}

func (s *Server) syncReconcilerConfig(ctx context.Context, reconcilerName string, configs []*protoapi.ReconcilerConfigSpec) error {
	cfg, err := s.querier.GetConfig(ctx, grpcreconcilersql.GetConfigParams{
		IncludeSecret:  false,
		ReconcilerName: reconcilerName,
	})
	if err != nil {
		return err
	}

	existing := make(map[string]struct{})
	for _, c := range cfg {
		existing[c.Key] = struct{}{}
	}

	// TODO: use transaction for upsert / delete
	for _, c := range configs {
		if err := s.querier.UpsertConfig(ctx, grpcreconcilersql.UpsertConfigParams{
			Reconciler:  reconcilerName,
			Key:         c.Key,
			DisplayName: c.DisplayName,
			Description: c.Description,
			Secret:      c.Secret,
		}); err != nil {
			return err
		}
		delete(existing, c.Key)
	}

	toDelete := make([]string, len(existing))
	for k := range existing {
		toDelete = append(toDelete, k)
	}

	return s.querier.DeleteConfig(ctx, grpcreconcilersql.DeleteConfigParams{
		Reconciler: reconcilerName,
		Keys:       toDelete,
	})
}

func toProtoReconcilerState(res *grpcreconcilersql.ReconcilerState) *protoapi.ReconcilerState {
	return &protoapi.ReconcilerState{
		Id:             res.ID.String(),
		ReconcilerName: res.ReconcilerName,
		TeamSlug:       string(res.TeamSlug),
		Value:          res.Value,
		CreatedAt:      timestamppb.New(res.CreatedAt.Time),
		UpdatedAt:      timestamppb.New(res.UpdatedAt.Time),
	}
}

func toProtoReconciler(rec *grpcreconcilersql.Reconciler) *protoapi.Reconciler {
	return &protoapi.Reconciler{
		Name:        rec.Name,
		Description: rec.Description,
		DisplayName: rec.DisplayName,
		Enabled:     rec.Enabled,
		MemberAware: rec.MemberAware,
	}
}
