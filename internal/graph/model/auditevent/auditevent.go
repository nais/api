package auditevent

import (
	"time"

	"github.com/nais/api/internal/database"

	"github.com/nais/api/internal/slug"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

type AuditEventNode interface {
	IsAuditEventNode()

	AuditEvent
}

type AuditEvent interface {
	IsAuditEvent()

	database.AuditEventInput
}

type AuditEventList struct {
	Nodes    []AuditEventNode `json:"nodes"`
	PageInfo model.PageInfo   `json:"pageInfo"`
}

type BaseAuditEvent struct {
	ID           scalar.Ident                 `json:"id"`
	Action       model.AuditEventAction       `json:"action"`
	Actor        string                       `json:"actor"`
	CreatedAt    time.Time                    `json:"createdAt"`
	Message      string                       `json:"message"`
	ResourceType model.AuditEventResourceType `json:"resourceType"`
	ResourceName string                       `json:"resourceName"`
	Team         slug.Slug                    `json:"team"`
}

func (e BaseAuditEvent) GetAction() string {
	return e.Action.String()
}

func (e BaseAuditEvent) GetActor() string {
	return e.Actor
}

func (e BaseAuditEvent) GetCreatedAt() time.Time {
	return e.CreatedAt
}

func (e BaseAuditEvent) GetData() any {
	return nil
}

func (e BaseAuditEvent) GetResourceType() string {
	return e.ResourceType.String()
}

func (e BaseAuditEvent) GetResourceName() string {
	return e.ResourceName
}

func (e BaseAuditEvent) GetTeam() slug.Slug {
	return e.Team
}

func (e BaseAuditEvent) WithMessage(message string) BaseAuditEvent {
	e.Message = message
	return e
}

func (BaseAuditEvent) IsAuditEvent() {}

func (BaseAuditEvent) IsAuditEventNode() {}
