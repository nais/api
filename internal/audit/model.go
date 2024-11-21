package audit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
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
	model.Node
	GetUUID() uuid.UUID
	ID() ident.Ident
}

type (
	AuditEntryConnection = pagination.Connection[AuditEntry]
	AuditEntryEdge       = pagination.Edge[AuditEntry]
)

type GenericAuditEntry struct {
	Actor           string            `json:"actor"`
	CreatedAt       time.Time         `json:"createdAt"`
	EnvironmentName *string           `json:"environmentName,omitempty"`
	Message         string            `json:"message"`
	ResourceType    AuditResourceType `json:"resourceType"`
	ResourceName    string            `json:"resourceName"`
	TeamSlug        *slug.Slug        `json:"teamSlug,omitempty"`
	Action          AuditAction       `json:"-"`
	UUID            uuid.UUID         `json:"-"`
	Data            []byte            `json:"-"`
}

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

// UnmarshalData unmarshals audit entry data. Its inverse is [MarshalData].
func UnmarshalData[T any](entry GenericAuditEntry) (*T, error) {
	var data T
	if err := json.Unmarshal(entry.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshaling audit entry data: %w", err)
	}

	return &data, nil
}

// TransformData unmarshals audit entry data and calls the provided transformer function with the data as argument.
func TransformData[T any](entry GenericAuditEntry, f func(*T) *T) (*T, error) {
	data, err := UnmarshalData[T](entry)
	if err != nil {
		return nil, err
	}

	return f(data), nil
}
