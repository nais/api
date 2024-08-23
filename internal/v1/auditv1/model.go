package auditv1

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

type (
	AuditLogResourceType string
	AuditLogAction       string
)

const (
	AuditLogActionAdded        AuditLogAction = "ADDED"
	AuditLogActionCreated      AuditLogAction = "CREATED"
	AuditLogActionDeleted      AuditLogAction = "DELETED"
	AuditLogActionRemoved      AuditLogAction = "REMOVED"
	AuditLogActionRestarted    AuditLogAction = "RESTARTED"
	AuditLogActionUpdated      AuditLogAction = "UPDATED"
	AuditLogActionSynchronized AuditLogAction = "SYNCHRONIZED"
)

type AuditEntry interface {
	modelv1.Node
	GetUUID() uuid.UUID
	ID() ident.Ident
	IsAuditLog()
}

type (
	AuditLogConnection = pagination.Connection[AuditEntry]
	AuditLogEdge       = pagination.Edge[AuditEntry]
)

type AuditLogGeneric struct {
	Action          AuditLogAction       `json:"action"`
	Actor           string               `json:"actor"`
	CreatedAt       time.Time            `json:"createdAt"`
	EnvironmentName *string              `json:"environmentName,omitempty"`
	Message         string               `json:"message"`
	ResourceType    AuditLogResourceType `json:"resourceType"`
	ResourceName    string               `json:"resourceName"`
	TeamSlug        *slug.Slug           `json:"teamSlug,omitempty"`
	UUID            uuid.UUID            `json:"-"`
}

func (AuditLogGeneric) IsAuditLog() {}

func (AuditLogGeneric) IsNode() {}

func (a AuditLogGeneric) ID() ident.Ident {
	return newIdent(a.UUID)
}

func (a AuditLogGeneric) GetUUID() uuid.UUID {
	return a.UUID
}

func (a AuditLogGeneric) WithMessage(message string) AuditLogGeneric {
	a.Message = message
	return a
}
