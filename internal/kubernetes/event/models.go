package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/event/eventsql"
)

type WorkloadEvent interface {
	model.Node
	IsWorkloadEvent()
	ID() ident.Ident
	GetUUID() uuid.UUID
}

type WorkloadEventBase struct {
	UUID        uuid.UUID
	TriggeredAt time.Time
	Message     string
}

func (WorkloadEventBase) IsNode()          {}
func (WorkloadEventBase) IsWorkloadEvent() {}

func (e WorkloadEventBase) GetUUID() uuid.UUID { return e.UUID }

func (e WorkloadEventBase) ID() ident.Ident {
	return ident.NewIdent(identKey, e.UUID.String())
}

type WorkloadSuccessfulRescaleEvent struct {
	WorkloadEventBase
	NewSize   int                                     `json:"newSize,string"`
	Direction WorkloadSuccessfulRescaleEventDirection `json:"direction"`
}

type WorkloadKillingEvent struct {
	WorkloadEventBase
	Container string `json:"container"`
	Reason    string `json:"reason"`
}

type WorkloadEventFilter struct {
	From *time.Time
	To   *time.Time
}

type (
	WorkloadEventConnection = pagination.Connection[WorkloadEvent]
	WorkloadEventEdge       = pagination.Edge[WorkloadEvent]
)

func toGraphEvent(row *eventsql.K8sEvent) (WorkloadEvent, error) {
	base := WorkloadEventBase{UUID: row.ID, TriggeredAt: row.TriggeredAt.Time}
	switch row.Reason {
	case "SuccessfulRescale":
		var event WorkloadSuccessfulRescaleEvent
		if err := json.Unmarshal(row.Data, &event); err != nil {
			return nil, err
		}
		if !event.Direction.IsValid() {
			return nil, fmt.Errorf("invalid workload rescale direction: %q", event.Direction)
		}
		base.Message = "Application scaled"
		event.WorkloadEventBase = base
		return &event, nil
	case "Killing":
		var event WorkloadKillingEvent
		if err := json.Unmarshal(row.Data, &event); err != nil {
			return nil, err
		}
		base.Message = "Instance restarted"
		event.WorkloadEventBase = base
		return &event, nil
	}
	return nil, fmt.Errorf("unsupported workload event reason: %q", row.Reason)
}
