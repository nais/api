package db

import (
	"context"

	"github.com/nais/api/internal/types"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/sqlc"
)

func (d *database) GetAuditLogsForTeam(ctx context.Context, slug slug.Slug, offset, limit int) ([]*AuditLog, int, error) {
	rows, err := d.querier.GetAuditLogsForTeam(ctx, sqlc.GetAuditLogsForTeamParams{
		TargetIdentifier: string(slug),
		Offset:           int32(offset),
		Limit:            int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}

	entries := make([]*AuditLog, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, &AuditLog{AuditLog: row})
	}

	total, err := d.querier.GetAuditLogsForTeamCount(ctx, string(slug))
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}

func (d *database) GetAuditLogsForReconciler(ctx context.Context, reconcilerName sqlc.ReconcilerName, offset, limit int) ([]*AuditLog, int, error) {
	rows, err := d.querier.GetAuditLogsForReconciler(ctx, sqlc.GetAuditLogsForReconcilerParams{
		TargetIdentifier: string(reconcilerName),
		Offset:           int32(offset),
		Limit:            int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}

	entries := make([]*AuditLog, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, &AuditLog{AuditLog: row})
	}

	total, err := d.querier.GetAuditLogsForReconcilerCount(ctx, string(reconcilerName))
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}

func (d *database) CreateAuditLogEntry(ctx context.Context, correlationID uuid.UUID, componentName types.ComponentName, actor *string, targetType types.AuditLogsTargetType, targetIdentifier string, action types.AuditAction, message string) error {
	return d.querier.CreateAuditLog(ctx, sqlc.CreateAuditLogParams{
		CorrelationID:    correlationID,
		Actor:            actor,
		ComponentName:    string(componentName),
		TargetType:       string(targetType),
		TargetIdentifier: targetIdentifier,
		Action:           string(action),
		Message:          message,
	})
}

func (d *database) GetAuditLogsForCorrelationID(ctx context.Context, correlationID uuid.UUID, offset, limit int) ([]*AuditLog, int, error) {
	rows, err := d.querier.GetAuditLogsForCorrelationID(ctx, sqlc.GetAuditLogsForCorrelationIDParams{
		CorrelationID: correlationID,
		Offset:        int32(offset),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}

	entries := make([]*AuditLog, len(rows))
	for i, row := range rows {
		entries[i] = &AuditLog{AuditLog: row}
	}
	total, err := d.querier.GetAuditLogsForCorrelationIDCount(ctx, correlationID)
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}
