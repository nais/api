package graph

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

func toGraphAuditLogs(logs []*database.AuditLog) []*model.AuditLog {
	graphLogs := make([]*model.AuditLog, len(logs))
	for i, log := range logs {
		graphLogs[i] = &model.AuditLog{
			ID:               scalar.AuditLogIdent(log.ID),
			Action:           log.Action,
			Actor:            log.Actor,
			ComponentName:    log.ComponentName,
			TargetType:       log.TargetType,
			CorrelationID:    log.CorrelationID.String(),
			TargetIdentifier: log.TargetIdentifier,
			Message:          log.Message,
			CreatedAt:        log.CreatedAt.Time,
		}
	}
	return graphLogs
}
