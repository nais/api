package audit

import (
	"time"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type AuditEventList struct {
	Nodes    []model.AuditEventNode `json:"nodes"`
	PageInfo model.PageInfo         `json:"pageInfo"`
}

// BaseAuditEvent is the base type for audit events.
type BaseAuditEvent struct {
	ID           scalar.Ident                 `json:"id"`
	Action       model.AuditEventAction       `json:"action"`
	Actor        string                       `json:"actor"`
	CreatedAt    time.Time                    `json:"createdAt"`
	Message      string                       `json:"message"`
	ResourceType model.AuditEventResourceType `json:"resourceType"`
	ResourceName string                       `json:"resourceName"`

	GQLVars BaseAuditEventGQLVars `json:"-"`
}

type BaseAuditEventGQLVars struct {
	Team        slug.Slug `json:"team"`
	Environment string    `json:"env"`
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

func (e BaseAuditEvent) GetTeam() *slug.Slug {
	if e.GQLVars.Team == "" {
		return nil
	}

	return &e.GQLVars.Team
}

func (e BaseAuditEvent) GetEnvironment() *string {
	if e.GQLVars.Environment == "" {
		return nil
	}

	return &e.GQLVars.Environment
}

func (e BaseAuditEvent) WithMessage(message string) BaseAuditEvent {
	e.Message = message
	return e
}

func (BaseAuditEvent) IsAuditEvent() {}

func (BaseAuditEvent) IsAuditEventNode() {}

type AuditEventTeamSetAlertsSlackChannel struct {
	BaseAuditEvent
	Data model.AuditEventTeamSetAlertsSlackChannelData
}

func (a AuditEventTeamSetAlertsSlackChannel) GetData() any {
	return a.Data
}

type AuditEventTeamSetDefaultSlackChannel struct {
	BaseAuditEvent
	Data model.AuditEventTeamSetDefaultSlackChannelData
}

func (a AuditEventTeamSetDefaultSlackChannel) GetData() any {
	return a.Data
}

type AuditEventTeamSetPurpose struct {
	BaseAuditEvent
	Data model.AuditEventTeamSetPurposeData
}

func (a AuditEventTeamSetPurpose) GetData() any {
	return a.Data
}

type AuditEventMemberAdded struct {
	BaseAuditEvent
	Data model.AuditEventMemberAddedData
}

func (a AuditEventMemberAdded) GetData() any {
	return a.Data
}

type AuditEventMemberRemoved struct {
	BaseAuditEvent
	Data model.AuditEventMemberRemovedData
}

func (a AuditEventMemberRemoved) GetData() any {
	return a.Data
}

type AuditEventMemberSetRole struct {
	BaseAuditEvent
	Data model.AuditEventMemberSetRoleData
}

func (a AuditEventMemberSetRole) GetData() any {
	return a.Data
}

type AuditEventTeamAddRepository struct {
	BaseAuditEvent
	Data model.AuditEventTeamAddRepositoryData
}

func (a AuditEventTeamAddRepository) GetData() any {
	return a.Data
}

type AuditEventTeamRemoveRepository struct {
	BaseAuditEvent
	Data model.AuditEventTeamRemoveRepositoryData
}

func (a AuditEventTeamRemoveRepository) GetData() any {
	return a.Data
}
