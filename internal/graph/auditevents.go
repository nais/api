package graph

import (
	audit "github.com/nais/api/internal/audit/events"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

func toGraphAuditEvents(events []audit.Event) []*model.AuditEvent {
	graphEvents := make([]*model.AuditEvent, len(events))
	for i, e := range events {
		graphEvents[i] = &model.AuditEvent{
			ID:           scalar.AuditEventIdent(e.ID()),
			Action:       e.Action(),
			Actor:        e.Actor(),
			Message:      e.Message(),
			CreatedAt:    e.CreatedAt(),
			ResourceType: model.AuditEventResourceType(e.ResourceType()),
		}
	}
	return graphEvents
}
