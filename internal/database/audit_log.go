package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger/audittype"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/slug"
)

type AuditLogsRepo interface {
	CreateAuditLogEntry(ctx context.Context, correlationID uuid.UUID, componentName logger.ComponentName, actor *string, targetType audittype.AuditLogsTargetType, targetIdentifier string, action audittype.AuditAction, message string) error
	GetAuditLogsForCorrelationID(ctx context.Context, correlationID uuid.UUID, offset, limit int) ([]*AuditLog, int, error)
	GetAuditLogsForReconciler(ctx context.Context, reconcilerName string, offset, limit int) ([]*AuditLog, int, error)
	GetAuditLogsForTeam(ctx context.Context, slug slug.Slug, offset, limit int) ([]*AuditLog, int, error)
}

type AuditLog struct {
	*sqlc.AuditLog
}

func (d *database) GetAuditLogsForTeam(ctx context.Context, slug slug.Slug, offset, limit int) ([]*AuditLog, int, error) {
	rows, err := d.querier.GetAuditLogsForTeam(ctx, string(slug), int32(offset), int32(limit))
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

func (d *database) GetAuditLogsForReconciler(ctx context.Context, reconcilerName string, offset, limit int) ([]*AuditLog, int, error) {
	rows, err := d.querier.GetAuditLogsForReconciler(ctx, reconcilerName, int32(offset), int32(limit))
	if err != nil {
		return nil, 0, err
	}

	entries := make([]*AuditLog, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, &AuditLog{AuditLog: row})
	}

	total, err := d.querier.GetAuditLogsForReconcilerCount(ctx, reconcilerName)
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}

func (d *database) CreateAuditLogEntry(ctx context.Context, correlationID uuid.UUID, componentName logger.ComponentName, actor *string, targetType audittype.AuditLogsTargetType, targetIdentifier string, action audittype.AuditAction, message string) error {
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
	rows, err := d.querier.GetAuditLogsForCorrelationID(ctx, correlationID, int32(offset), int32(limit))
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
