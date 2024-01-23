package grpc

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ReconcilerResourcesServer struct {
	db interface {
		database.ReconcilerResourceRepo
		database.TeamRepo
	}
	protoapi.UnimplementedReconcilerResourcesServer
}

func (r *ReconcilerResourcesServer) Create(ctx context.Context, in *protoapi.SaveReconcilerResourceRequest) (*protoapi.SaveReconcilerResourceResponse, error) {
	switch {
	case in.ReconcilerName == "":
		return nil, status.Error(400, "reconcilerName is required")
	case in.TeamSlug == "":
		return nil, status.Error(400, "teamSlug is required")
	}

	slg := slug.Slug(in.TeamSlug)
	rn := in.ReconcilerName

	for _, rr := range in.Resources {
		switch {
		case rr.Name == "":
			return nil, status.Error(400, "name is required")
		case rr.Value == "":
			return nil, status.Error(400, "value is required")
		}
		_, err := r.db.CreateReconcilerResource(ctx, rn, slg, rr.Name, rr.Value, rr.Metadata)
		if err != nil {
			return nil, err
		}
	}

	return &protoapi.SaveReconcilerResourceResponse{}, nil
}

func (r *ReconcilerResourcesServer) List(ctx context.Context, req *protoapi.ListReconcilerResourcesRequest) (*protoapi.ListReconcilerResourcesResponse, error) {
	var teamSlug *slug.Slug

	if req.TeamSlug != "" {
		slg := slug.Slug(req.TeamSlug)
		teamSlug = &slg
	}

	limit, offset := pagination(req)
	total := 0
	res, err := r.db.GetReconcilerResources(ctx, req.ReconcilerName, teamSlug, offset, limit)
	if err != nil {
		return nil, err
	}

	resp := &protoapi.ListReconcilerResourcesResponse{
		PageInfo: pageInfo(req, total),
	}
	for _, rr := range res {
		resp.Nodes = append(resp.Nodes, toProtoReconcilerResource(rr))
	}
	return nil, nil
}

func toProtoReconcilerResource(res *database.ReconcilerResource) *protoapi.ReconcilerResource {
	return &protoapi.ReconcilerResource{
		Id:             res.ID.String(),
		ReconcilerName: res.ReconcilerName,
		TeamSlug:       string(res.TeamSlug),
		Name:           res.Name,
		Value:          res.Value,
		Metadata:       res.Metadata,
		CreatedAt:      timestamppb.New(res.CreatedAt.Time),
		UpdatedAt:      timestamppb.New(res.UpdatedAt.Time),
	}
}
