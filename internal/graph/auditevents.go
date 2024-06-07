package graph

import (
	"github.com/nais/api/internal/auditevent"
	"k8s.io/utils/ptr"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

func toGraphAuditEvents(events []auditevent.Event) []*model.AuditEvent {
	graphEvents := make([]*model.AuditEvent, len(events))
	for i, e := range events {
		graphEvents[i] = &model.AuditEvent{
			ID:           scalar.AuditEventIdent(e.ID()),
			Action:       e.Action(),
			Actor:        ptr.To(e.Actor()),
			Message:      e.Message(),
			CreatedAt:    e.CreatedAt(),
			ResourceType: e.ResourceType(),
		}
	}
	return graphEvents
}
