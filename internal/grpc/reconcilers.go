package grpc

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/pkg/protoapi"
	"k8s.io/utils/ptr"
)

type ReconcilersServer struct {
	db database.ReconcilerRepo
	protoapi.UnimplementedReconcilersServer
}

func (r *ReconcilersServer) SetReconcilerErrorForTeam(ctx context.Context, in *protoapi.SetReconcilerErrorForTeamRequest) (*protoapi.SetReconcilerErrorForTeamResponse, error) {
	panic("not implemented")
}

func (r *ReconcilersServer) SuccessfulTeamSync(ctx context.Context, in *protoapi.SuccessfulTeamSyncRequest) (*protoapi.SuccessfulTeamSyncResponse, error) {
	panic("not implemented")
}

func (r *ReconcilersServer) Register(ctx context.Context, req *protoapi.RegisterReconcilerRequest) (*protoapi.RegisterReconcilerResponse, error) {
	for _, rec := range req.Reconcilers {
		if _, err := r.db.UpsertReconciler(ctx, rec.Name, rec.DisplayName, rec.Description, rec.MemberAware, rec.EnableByDefault); err != nil {
			return nil, err
		}

		if err := r.db.SyncReconcilerConfig(ctx, rec.Name, rec.Config); err != nil {
			return nil, err
		}
	}

	return &protoapi.RegisterReconcilerResponse{}, nil
}

func (r *ReconcilersServer) Get(ctx context.Context, req *protoapi.GetReconcilerRequest) (*protoapi.GetReconcilerResponse, error) {
	rec, err := r.db.GetReconciler(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return &protoapi.GetReconcilerResponse{
		Reconciler: toProtoReconciler(rec),
	}, nil
}

func (r *ReconcilersServer) List(ctx context.Context, req *protoapi.ListReconcilersRequest) (*protoapi.ListReconcilersResponse, error) {
	limit, offset := pagination(req)
	recs, total, err := r.db.GetReconcilers(ctx, database.Page{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
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

func (r *ReconcilersServer) Config(ctx context.Context, req *protoapi.ConfigReconcilerRequest) (*protoapi.ConfigReconcilerResponse, error) {
	cfg, err := r.db.GetReconcilerConfig(ctx, req.ReconcilerName)
	if err != nil {
		return nil, err
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

func toProtoReconciler(rec *database.Reconciler) *protoapi.Reconciler {
	return &protoapi.Reconciler{
		Name:        rec.Name,
		Description: rec.Description,
		DisplayName: rec.DisplayName,
		Enabled:     rec.Enabled,
		MemberAware: rec.MemberAware,
	}
}
