package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuditLogsServer struct {
	auditlog auditlogger.AuditLogger
	db       database.ReconcilerRepo
	protoapi.UnimplementedAuditLogsServer
}

func (a *AuditLogsServer) Create(ctx context.Context, r *protoapi.CreateAuditLogsRequest) (*protoapi.CreateAuditLogsResponse, error) {
	if _, err := a.db.GetReconciler(ctx, r.ReconcilerName); errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "reconciler not found")
	} else if err != nil {
		return nil, status.Error(codes.Internal, "failed to get reconciler")
	}

	targets := []auditlogger.Target{
		auditlogger.ReconcilerTarget(r.ReconcilerName),
	}
	for _, t := range r.Targets {
		switch t := t.AuditLogTargetType.(type) {
		case *protoapi.AuditLogTarget_TeamSlug:
			targets = append(targets, auditlogger.TeamTarget(slug.Slug(t.TeamSlug)))
		case *protoapi.AuditLogTarget_User:
			targets = append(targets, auditlogger.UserTarget(t.User))
		case *protoapi.AuditLogTarget_Generic:
			targets = append(targets, auditlogger.Target{Identifier: t.Generic})
		default:
			return nil, status.Error(codes.InvalidArgument, "invalid target type")
		}
	}

	correlationID, err := uuid.Parse(r.CorrelationId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid correlation id")
	}

	fields := auditlogger.Fields{
		Action:        audittype.AuditAction(r.Action),
		CorrelationID: correlationID,
	}

	a.auditlog.Logf(ctx, targets, fields, r.Message)

	return &protoapi.CreateAuditLogsResponse{}, nil
}
