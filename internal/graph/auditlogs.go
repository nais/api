package graph

import (
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/logger"
)

func toGraphAuditLogs(logs []*database.AuditLog) []*model.AuditLog {
	graphLogs := make([]*model.AuditLog, 0, len(logs))
	for _, log := range logs {
		graphLogs = append(graphLogs, &model.AuditLog{
			ID:               scalar.AuditLogIdent(log.ID),
			Action:           audittype.AuditAction(log.Action),
			Actor:            log.Actor,
			ComponentName:    logger.ComponentName(log.ComponentName),
			TargetType:       audittype.AuditLogsTargetType(log.TargetType),
			CorrelationID:    log.CorrelationID.String(),
			TargetIdentifier: log.TargetIdentifier,
			Message:          log.Message,
			CreatedAt:        log.CreatedAt.Time,
		})
	}
	return graphLogs
}
