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
	AuditResourceType string
	AuditAction       string
)

const (
	AuditActionAdded        AuditAction = "ADDED"
	AuditActionCreated      AuditAction = "CREATED"
	AuditActionDeleted      AuditAction = "DELETED"
	AuditActionRemoved      AuditAction = "REMOVED"
	AuditActionRestarted    AuditAction = "RESTARTED"
	AuditActionUpdated      AuditAction = "UPDATED"
	AuditActionSynchronized AuditAction = "SYNCHRONIZED"
)

type AuditEntry interface {
	modelv1.Node
	GetUUID() uuid.UUID
	ID() ident.Ident
	IsAuditLog()
}

type (
	AuditEntryConnection = pagination.Connection[AuditEntry]
	AuditEntryEdge       = pagination.Edge[AuditEntry]
)

type GenericAuditEntry struct {
	Action          AuditAction       `json:"action"`
	Actor           string            `json:"actor"`
	CreatedAt       time.Time         `json:"createdAt"`
	EnvironmentName *string           `json:"environmentName,omitempty"`
	Message         string            `json:"message"`
	ResourceType    AuditResourceType `json:"resourceType"`
	ResourceName    string            `json:"resourceName"`
	TeamSlug        *slug.Slug        `json:"teamSlug,omitempty"`
	UUID            uuid.UUID         `json:"-"`
	Data            []byte            `json:"-"`
}

func (GenericAuditEntry) IsAuditLog() {}

func (GenericAuditEntry) IsNode() {}

func (a GenericAuditEntry) ID() ident.Ident {
	return newIdent(a.UUID)
}

func (a GenericAuditEntry) GetUUID() uuid.UUID {
	return a.UUID
}

func (a GenericAuditEntry) WithMessage(message string) GenericAuditEntry {
	a.Message = message
	return a
}
