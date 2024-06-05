package graph

import (
	"fmt"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

func toGraphAuditEvents(events []*database.AuditEvent) []*model.AuditEvent {
	graphEvents := make([]*model.AuditEvent, len(events))
	for i, e := range events {
		graphEvents[i] = &model.AuditEvent{
			ID:           scalar.AuditEventIdent(e.ID),
			Action:       e.Action,
			Actor:        &e.Actor,
			Message:      fmt.Sprintf("yolo: %s - %s", e.Action, e.Actor), // TODO: figure out how to construct message
			CreatedAt:    e.CreatedAt.Time,
			ResourceType: e.ResourceType,
		}
	}
	return graphEvents
}
