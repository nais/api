package event

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

type (
	AuditLogResourceType string
	AuditLogAction       string
)

func (e AuditLogAction) String() string {
	return string(e)
}

func (e *AuditLogAction) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = AuditLogAction(str)
	return nil
}

func (e AuditLogAction) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type AuditLog interface {
	GetAction() string
	GetActor() string
	GetData() any
	GetEnvironment() *string
	GetResourceType() string
	GetResourceName() string
	GetTeam() *slug.Slug
}

type (
	EventConnection = pagination.Connection[*AuditLog]
	EventEdge       = pagination.Edge[*AuditLog]
)

type AuditEventList struct {
	Nodes    []model.AuditEventNode `json:"nodes"`
	PageInfo model.PageInfo         `json:"pageInfo"`
}

// BaseAuditLog is the base type for audit events.
type BaseAuditLog struct {
	ID        scalar.Ident   `json:"id"`
	Action    AuditLogAction `json:"action"`
	Actor     string         `json:"actor"`
	CreatedAt time.Time      `json:"createdAt"`
	Message   string         `json:"message"`

	ResourceIdent   ident.Ident `json:"-"`
	TeamSlug        slug.Slug   `json:"-"`
	EnvironmentName string      `json:"-"`
}

func (e BaseAuditLog) GetAction() string {
	return e.Action.String()
}

func (e BaseAuditLog) GetActor() string {
	return e.Actor
}

func (e BaseAuditLog) GetCreatedAt() time.Time {
	return e.CreatedAt
}

func (e BaseAuditLog) GetData() any {
	return nil
}

func (e BaseAuditLog) GetTeam() *slug.Slug {
	if e.TeamSlug == "" {
		return nil
	}

	return &e.TeamSlug
}

func (e BaseAuditLog) GetEnvironment() *string {
	if e.EnvironmentName == "" {
		return nil
	}

	return &e.EnvironmentName
}

func (e BaseAuditLog) WithMessage(message string) BaseAuditLog {
	e.Message = message
	return e
}

func (BaseAuditLog) IsAuditEvent() {}

func (BaseAuditLog) IsAuditEventNode() {}
