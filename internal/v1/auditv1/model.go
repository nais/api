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

type AuditLog interface {
	modelv1.Node
	GetAction() string
	GetActor() string
	GetData() any
	GetEnvironmentName() *string
	GetUUID() uuid.UUID
	GetResourceType() string
	GetResourceName() string
	GetTeamSlug() *slug.Slug
}

type (
	AuditLogConnection = pagination.Connection[AuditLog]
	AuditLogEdge       = pagination.Edge[AuditLog]
)

type AuditLogGeneric struct {
	Action          AuditLogAction       `json:"action"`
	Actor           string               `json:"actor"`
	CreatedAt       time.Time            `json:"createdAt"`
	EnvironmentName *string              `json:"environmentName"`
	Message         string               `json:"message"`
	ResourceType    AuditLogResourceType `json:"resourceType"`
	ResourceName    string               `json:"resourceName"`
	TeamSlug        *slug.Slug           `json:"teamSlug"`

	UUID uuid.UUID `json:"-"`
}

func (AuditLogGeneric) IsAuditLog() {}

func (AuditLogGeneric) IsNode() {}

func (a AuditLogGeneric) ID() ident.Ident {
	return newIdent(a.UUID)
}

func (a AuditLogGeneric) GetAction() string {
	return string(a.Action)
}

func (a AuditLogGeneric) GetActor() string {
	return a.Actor
}

func (a AuditLogGeneric) GetCreatedAt() time.Time {
	return a.CreatedAt
}

func (a AuditLogGeneric) GetData() any {
	return nil
}

func (a AuditLogGeneric) GetResourceType() string {
	return string(a.ResourceType)
}

func (a AuditLogGeneric) GetResourceName() string {
	return a.ResourceName
}

func (a AuditLogGeneric) GetTeamSlug() *slug.Slug {
	return a.TeamSlug
}

func (a AuditLogGeneric) GetEnvironmentName() *string {
	return a.EnvironmentName
}

func (a AuditLogGeneric) GetUUID() uuid.UUID {
	return a.UUID
}

func (a AuditLogGeneric) WithMessage(message string) AuditLogGeneric {
	a.Message = message
	return a
}
